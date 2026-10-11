package npmprofile

import (
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

func TestHardenedClientClonesTLSPolicy(t *testing.T) {
	t.Parallel()
	for _, minimum := range []uint16{0, tls.VersionTLS12, tls.VersionTLS13} {
		t.Run(tls.VersionName(minimum), func(t *testing.T) {
			t.Parallel()
			transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: minimum}}
			original := &http.Client{Transport: transport, Timeout: time.Second}
			got, err := hardenedHTTPClient(original)
			if err != nil {
				t.Fatal(err)
			}
			defer got.CloseIdleConnections()
			cloned, ok := got.Transport.(*http.Transport)
			if !ok {
				t.Fatalf("transport = %T; want *http.Transport", got.Transport)
			}
			if got == original || cloned == transport || cloned.TLSClientConfig == transport.TLSClientConfig || transport.TLSClientConfig.MinVersion != minimum {
				t.Fatal("hardening mutated or reused caller-owned state")
			}
			if cloned.TLSClientConfig.MinVersion != max(minimum, tls.VersionTLS12) || got.Timeout != time.Second {
				t.Fatalf("TLS minimum/timeout = %d/%v", cloned.TLSClientConfig.MinVersion, got.Timeout)
			}
		})
	}
}

func TestHardenedClientDefaultTransportRejections(t *testing.T) {
	original := http.DefaultTransport
	http.DefaultTransport = fetchForbiddenTransport{}
	t.Cleanup(func() { http.DefaultTransport = original })
	for _, client := range []*http.Client{nil, {}} {
		got, err := hardenedHTTPClient(client)
		if got != nil || err == nil || err.Error() != "default HTTP transport has an unsupported type" {
			t.Fatalf("hardenedHTTPClient() = %v, %v; want unsupported default transport", got, err)
		}
	}
}

func TestHTTPBoundedResponseContracts(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, body, want string
		status           int
		calls            int32
	}{
		{"exact bound", "abc", "", http.StatusOK, 1},
		{"one byte over", "abcd", "authenticated HTTP response exceeds size limit", http.StatusOK, 1},
		{"terminal status", "bad", "", http.StatusBadRequest, 1},
		{"retry exhausted", "bad", "", http.StatusServiceUnavailable, 3},
		{"truncated response", "abc", "authenticated HTTP response could not be read", http.StatusOK, 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != http.MethodGet || r.URL.Path != "/metadata" || r.Header.Get("Authorization") != "Bearer test-only" {
					t.Errorf("request = %s %s %v", r.Method, r.URL, r.Header)
				}
				if test.name == "truncated response" {
					w.Header().Set("Content-Length", "9")
				}
				w.WriteHeader(test.status)
				if _, err := w.Write([]byte(test.body)); err != nil {
					t.Errorf("write response: %v", err)
				}
			}))
			defer server.Close()
			endpoint, err := url.Parse(server.URL + "/metadata")
			if err != nil {
				t.Fatal(err)
			}
			got, err := performBoundedRequest(t.Context(), server.Client(), http.MethodGet, endpoint, http.Header{"Authorization": {"Bearer test-only"}}, 3)
			if calls.Load() != test.calls {
				t.Fatalf("requests = %d; want %d", calls.Load(), test.calls)
			}
			if test.want != "" {
				if err == nil || err.Error() != test.want || got.body != nil {
					t.Fatalf("bounded response = %#v, %v; want %q", got, err, test.want)
				}
			} else if err != nil || got.status != test.status || string(got.body) != test.body {
				t.Fatalf("bounded response = %#v, %v; want status %d and %q", got, err, test.status, test.body)
			}
		})
	}
}

func TestHTTPBoundedInvalidMethod(t *testing.T) {
	t.Parallel()
	_, err := performBoundedRequest(t.Context(), &http.Client{Transport: fetchForbiddenTransport{}}, "bad method", &url.URL{Scheme: "https", Host: "example.invalid"}, nil, 3)
	if err == nil || err.Error() != "construct authenticated HTTP request" {
		t.Fatalf("invalid method error = %v", err)
	}
}

func TestHTTPProxyContracts(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, want string
		proxy      func(*http.Request) (*url.URL, error)
	}{
		{"no proxy hook", "", nil},
		{"no proxy selected", "", func(*http.Request) (*url.URL, error) { return nil, nil }},
		{"anonymous proxy", "", func(*http.Request) (*url.URL, error) { return &url.URL{Scheme: "https", Host: "proxy.invalid"}, nil }},
		{"credentials", "credential-bearing HTTPS proxies are prohibited", func(*http.Request) (*url.URL, error) {
			return &url.URL{Scheme: "https", Host: "proxy.invalid", User: url.UserPassword("user", "secret")}, nil
		}},
		{"resolver failure", "resolve HTTPS proxy", func(*http.Request) (*url.URL, error) { return nil, errors.New("secret resolver detail") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(http.MethodGet, "https://example.invalid/", nil)
			err := rejectCredentialBearingProxy(&http.Client{Transport: &http.Transport{Proxy: test.proxy}}, request)
			if test.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || err.Error() != test.want {
				t.Fatalf("proxy error = %v; want %q", err, test.want)
			}
		})
	}
}
