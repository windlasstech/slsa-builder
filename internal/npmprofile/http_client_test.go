package npmprofile

import (
	"crypto/tls"
	"net/http"
	"testing"
)

func TestInjectedTransportIsHardened(t *testing.T) {
	t.Parallel()

	injected := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS10}}
	client, err := hardenedHTTPClient(&http.Client{Transport: injected})
	if err != nil {
		t.Fatal(err)
	}
	hardened, ok := client.Transport.(*http.Transport)
	if !ok || hardened == injected || hardened.TLSClientConfig.MinVersion < tls.VersionTLS12 {
		t.Fatalf("transport was not cloned and hardened: %#v", client.Transport)
	}
	if injected.TLSClientConfig.MinVersion != tls.VersionTLS10 {
		t.Fatal("hardening mutated the caller-owned transport")
	}
	if _, err := hardenedHTTPClient(&http.Client{
		Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) { return nil, nil }),
	}); err == nil {
		t.Fatal("hardenedHTTPClient() accepted an unauditable custom transport")
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (function roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
