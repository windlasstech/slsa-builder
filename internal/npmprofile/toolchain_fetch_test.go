package npmprofile

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
)

// fetchHTTPS returns plain wrapped errors; command adapters classify them.
// Request URLs are internally constructed distribution URLs, not external input;
// credential rejection here applies to proxies rather than request URL userinfo.
func TestFetchHTTPSResponses(t *testing.T) {
	for _, test := range []struct {
		name    string
		status  int
		body    string
		maximum int64
		wantErr string
	}{
		{name: "success at size bound", status: http.StatusOK, body: "bytes", maximum: 5},
		{name: "one byte over bound", status: http.StatusOK, body: "bytes!", maximum: 5, wantErr: "distribution response exceeds size limit"},
		{name: "HTTP error", status: http.StatusNotFound, maximum: 5, wantErr: "distribution endpoint returned HTTP 404"},
		{name: "redirect refused", status: http.StatusFound, maximum: 5, wantErr: "distribution endpoint redirect is prohibited"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			server := newFetchTLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.URL.Path != "/distribution" {
					t.Errorf("request path = %q; redirect must not be followed", r.URL.Path)
				}
				if r.Method != http.MethodGet || r.Header.Get("Accept") != "application/octet-stream" || r.Header.Get("User-Agent") != "windlass-slsa-builder" {
					t.Errorf("unexpected distribution request: method=%s headers=%v", r.Method, r.Header)
				}
				w.Header().Set("Location", "/redirect-target")
				w.WriteHeader(test.status)
				if _, err := w.Write([]byte(test.body)); err != nil {
					t.Errorf("write distribution: %v", err)
				}
			}))
			got, err := fetchHTTPS(t.Context(), server.URL+"/distribution", test.maximum, "application/octet-stream")
			if test.wantErr == "" {
				if err != nil || string(got) != test.body {
					t.Fatalf("fetchHTTPS() = %q, %v; want %q, nil", got, err, test.body)
				}
			} else {
				var urlErr *url.Error
				cause := err
				if errors.As(err, &urlErr) {
					cause = urlErr.Err
				}
				if cause == nil || cause.Error() != test.wantErr || got != nil {
					t.Fatalf("fetchHTTPS() = %q, %v; want nil, %q", got, err, test.wantErr)
				}
			}
			wantCalls := int32(1)
			if test.status != http.StatusOK {
				wantCalls = 3 // Terminal HTTP/redirect errors exhaust the bounded retry loop.
			}
			if got := calls.Load(); got != wantCalls {
				t.Fatalf("requests = %d; want %d", got, wantCalls)
			}
		})
	}
}

func TestFetchHTTPSRetryThenSuccess(t *testing.T) {
	var calls atomic.Int32
	server := newFetchTLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("retry method = %s; want GET", r.Method)
		}
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if _, err := w.Write([]byte("recovered")); err != nil {
			t.Errorf("write recovered response: %v", err)
		}
	}))
	got, err := fetchHTTPS(t.Context(), server.URL, 9, "application/octet-stream")
	if err != nil || string(got) != "recovered" || calls.Load() != 2 {
		t.Fatalf("fetchHTTPS() = %q, %v with %d requests; want recovered, nil and 2 requests", got, err, calls.Load())
	}
}

func TestFetchHTTPSPrecancelled(t *testing.T) {
	server := newFetchTLSServer(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("pre-cancelled request reached server")
	}))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	got, err := fetchHTTPS(ctx, server.URL, 5, "application/octet-stream")
	if !errors.Is(err, context.Canceled) || got != nil {
		t.Fatalf("fetchHTTPS() = %q, %v; want nil, context.Canceled", got, err)
	}
}

func TestFetchHTTPSRejectsTransportConfiguration(t *testing.T) {
	for _, test := range []struct {
		name      string
		transport http.RoundTripper
		wantErr   string
	}{
		{name: "unsupported transport", transport: fetchForbiddenTransport{}, wantErr: "default HTTP transport has an unsupported type"},
		{name: "credential proxy", transport: &http.Transport{Proxy: func(*http.Request) (*url.URL, error) {
			return &url.URL{Scheme: "https", Host: "127.0.0.1:1", User: url.UserPassword("user", "secret")}, nil
		}}, wantErr: "credential-bearing HTTPS proxies are prohibited"},
		{name: "proxy resolution failure", transport: &http.Transport{Proxy: func(*http.Request) (*url.URL, error) {
			return nil, errors.New("proxy resolution failed")
		}}, wantErr: "resolve HTTPS proxy: proxy resolution failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			original := http.DefaultTransport
			http.DefaultTransport = test.transport
			t.Cleanup(func() { http.DefaultTransport = original })
			got, err := fetchHTTPS(t.Context(), "https://127.0.0.1:1/", 5, "application/octet-stream")
			if err == nil || err.Error() != test.wantErr || got != nil {
				t.Fatalf("fetchHTTPS() = %q, %v; want nil, %q", got, err, test.wantErr)
			}
		})
	}
}

type fetchForbiddenTransport struct{}

func (fetchForbiddenTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("network is forbidden")
}

func newFetchTLSServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	address := server.Listener.Addr().String()
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		t.Fatalf("split test listener address: %v", err)
	}
	// fetchHTTPS replaces TLSClientConfig, but Clone preserves this dial seam.
	// Trust only this server's certificate and refuse every non-loopback target.
	transport := &http.Transport{DialTLSContext: func(ctx context.Context, network, target string) (net.Conn, error) {
		if target != address {
			return nil, fmt.Errorf("unexpected TLS target %q", target)
		}
		dialer := tls.Dialer{Config: &tls.Config{RootCAs: roots, ServerName: host, MinVersion: tls.VersionTLS12}}
		return dialer.DialContext(ctx, network, address)
	}}
	original := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() {
		http.DefaultTransport = original
		transport.CloseIdleConnections()
	})
	return server
}
