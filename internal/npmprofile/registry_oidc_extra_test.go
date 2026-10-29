package npmprofile

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRegistryClientRejectsUnsupportedTransport pins the fail-closed
// constructor: an HTTP client whose transport is not a *http.Transport
// cannot be hardened, so construction errors instead of downgrading policy.
func TestRegistryClientRejectsUnsupportedTransport(t *testing.T) {
	t.Parallel()
	injected := &http.Client{Transport: fetchForbiddenTransport{}}
	if _, err := NewRegistryClient(RegistryClientConfig{HTTPClient: injected, RegistryURL: "https://registry.npmjs.org/"}); err == nil {
		t.Fatal("NewRegistryClient() accepted an unsupported transport")
	}
}

// TestRegistryClientURLNormalization pins the normalized-root contract: the
// default HTTPS port is stripped, and a nil client renders an empty URL.
func TestRegistryClientURLNormalization(t *testing.T) {
	t.Parallel()

	t.Run("default port stripped", func(t *testing.T) {
		t.Parallel()
		client, err := NewRegistryClient(RegistryClientConfig{RegistryURL: "https://registry.npmjs.org:443/"})
		if err != nil {
			t.Fatal(err)
		}
		if got := client.URL(); got != "https://registry.npmjs.org/" {
			t.Fatalf("URL() = %q, want %q", got, "https://registry.npmjs.org/")
		}
	})

	t.Run("nil client renders empty URL", func(t *testing.T) {
		t.Parallel()
		var client *RegistryClient
		if got := client.URL(); got != "" {
			t.Fatalf("URL() = %q, want empty for a nil client", got)
		}
	})
}

// TestRegistryPreflightHTTPError pins the unclassifiable-status contract: a
// packument response other than 200 or 404 surfaces the status rather than a
// fabricated existence state.
func TestRegistryPreflightHTTPError(t *testing.T) {
	t.Parallel()
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", request.Method)
		}
		writer.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()
	client, err := NewRegistryClient(RegistryClientConfig{HTTPClient: server.Client(), RegistryURL: server.URL + "/"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Preflight(t.Context(), "safe-package", "1.0.0"); err == nil ||
		!strings.Contains(err.Error(), "registry packument returned HTTP 400") {
		t.Fatalf("Preflight() error = %v, want the HTTP 400 classification", err)
	}
}

// TestRegistryPreflightVersionIdentityMismatch pins the exact-version
// identity check: a version entry whose embedded name or version disagrees
// with the requested coordinates fails closed as malformed.
func TestRegistryPreflightVersionIdentityMismatch(t *testing.T) {
	t.Parallel()
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", request.Method)
		}
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, `{"name":"safe-package","versions":{"1.0.0":{"name":"other-package","version":"1.0.0","dist":{"integrity":"sha512-YQ==","tarball":"https://registry.npmjs.org/example.tgz"}}}}`)
	}))
	defer server.Close()
	client, err := NewRegistryClient(RegistryClientConfig{HTTPClient: server.Client(), RegistryURL: server.URL + "/"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Preflight(t.Context(), "safe-package", "1.0.0"); err == nil ||
		!strings.Contains(err.Error(), "registry version metadata identity is malformed") {
		t.Fatalf("Preflight() error = %v, want the version-identity rejection", err)
	}
}

// TestRegistryAttestationsRejections pins the attestation reader's failure
// forms: unusable inputs, a failed request, and an unclassifiable status all
// error without fabricating an attestation state.
func TestRegistryAttestationsRejections(t *testing.T) {
	t.Parallel()

	t.Run("nil client", func(t *testing.T) {
		t.Parallel()
		var client *RegistryClient
		if _, err := client.Attestations(t.Context(), "safe-package", "1.0.0"); err == nil {
			t.Fatal("Attestations() error = nil for a nil client")
		}
	})

	t.Run("invalid package name", func(t *testing.T) {
		t.Parallel()
		client, err := NewRegistryClient(RegistryClientConfig{RegistryURL: "https://registry.npmjs.org/"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := client.Attestations(t.Context(), "foo bar", "1.0.0"); err == nil {
			t.Fatal("Attestations() accepted an invalid package name")
		}
	})

	t.Run("request failure", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		client, err := NewRegistryClient(RegistryClientConfig{HTTPClient: server.Client(), RegistryURL: server.URL + "/"})
		if err != nil {
			t.Fatal(err)
		}
		server.Close()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if _, err := client.Attestations(ctx, "safe-package", "1.0.0"); err == nil ||
			!strings.Contains(err.Error(), "registry attestation request failed") {
			t.Fatalf("Attestations() error = %v, want the request-failure classification", err)
		}
	})

	t.Run("unknown element member", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.Method != http.MethodGet {
				t.Errorf("method = %s, want GET", request.Method)
			}
			writer.Header().Set("Content-Type", "application/json")
			fmt.Fprint(writer, `{"attestations":[{"predicateType":"https://slsa.dev/provenance/v1","bundle":{"exact":"bytes"},"unexpectedMember":1}]}`)
		}))
		defer server.Close()
		client, err := NewRegistryClient(RegistryClientConfig{HTTPClient: server.Client(), RegistryURL: server.URL + "/"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := client.Attestations(t.Context(), "safe-package", "1.0.0"); err == nil ||
			!strings.Contains(err.Error(), "registry attestation response has an invalid shape") {
			t.Fatalf("Attestations() error = %v, want the invalid-shape rejection", err)
		}
	})

	t.Run("unclassifiable status", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.Method != http.MethodGet {
				t.Errorf("method = %s, want GET", request.Method)
			}
			writer.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()
		client, err := NewRegistryClient(RegistryClientConfig{HTTPClient: server.Client(), RegistryURL: server.URL + "/"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := client.Attestations(t.Context(), "safe-package", "1.0.0"); err == nil ||
			!strings.Contains(err.Error(), "registry attestation endpoint returned HTTP 400") {
			t.Fatalf("Attestations() error = %v, want the HTTP 400 classification", err)
		}
	})
}

// TestNewOIDCClientRejections pins the fail-closed OIDC constructor: the
// registry URL must pass the hardened root validation and the injected HTTP
// client must be hardenable.
func TestNewOIDCClientRejections(t *testing.T) {
	t.Parallel()

	t.Run("insecure registry URL", func(t *testing.T) {
		t.Parallel()
		if _, err := NewOIDCClient(OIDCClientConfig{RegistryURL: "http://registry.npmjs.org/"}); err == nil {
			t.Fatal("NewOIDCClient() accepted an insecure registry URL")
		}
	})

	t.Run("unsupported transport", func(t *testing.T) {
		t.Parallel()
		injected := &http.Client{Transport: fetchForbiddenTransport{}}
		if _, err := NewOIDCClient(OIDCClientConfig{HTTPClient: injected, RegistryURL: "https://registry.npmjs.org/"}); err == nil {
			t.Fatal("NewOIDCClient() accepted an unsupported transport")
		}
	})
}

// TestOIDCAcquireIdentityUnusableEndpoint pins the capability-endpoint
// contract: a non-HTTPS (or otherwise malformed) GitHub OIDC request endpoint
// reports the capability as unavailable before any request is attempted.
func TestOIDCAcquireIdentityUnusableEndpoint(t *testing.T) {
	t.Parallel()
	client, err := NewOIDCClient(OIDCClientConfig{
		RegistryURL:         "https://registry.npmjs.org/",
		IDTokenRequestURL:   "http://insecure.example.invalid/token",
		IDTokenRequestToken: testIdentityToken,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, report := client.AcquireIdentity(t.Context())
	assertReportPrimary(t, report, IDOIDCCapabilityUnavailable)
}

// TestOIDCAcquireIdentityCallerWorkflowMismatch pins the trusted-publisher
// binding: when the issued token's workflow claim conflicts with the observed
// caller workflow, acquisition reports the trusted-publisher mismatch.
func TestOIDCAcquireIdentityCallerWorkflowMismatch(t *testing.T) {
	t.Parallel()
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"workflow_ref":"windlasstech/other/.github/workflows/release.yml@refs/tags/v9.9.9"}`))
	jwt := "header." + payload + ".signature"
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", request.Method)
		}
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(writer, `{"value":%q}`, jwt)
	}))
	defer server.Close()
	client, err := NewOIDCClient(OIDCClientConfig{
		HTTPClient:          server.Client(),
		RegistryURL:         server.URL + "/",
		IDTokenRequestURL:   server.URL + "/identity",
		IDTokenRequestToken: testIdentityToken,
		GitHubWorkflowRef:   "windlasstech/caller/.github/workflows/release.yml@refs/tags/v1.2.3",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, report := client.AcquireIdentity(t.Context())
	assertReportPrimary(t, report, IDTrustedPublisherMismatch)
}

// TestOIDCExchangeInputRejections pins the fail-closed exchange inputs: an
// invalid package name or an empty identity token yields the indeterminate
// classification without any registry request.
func TestOIDCExchangeInputRejections(t *testing.T) {
	t.Parallel()
	client, err := NewOIDCClient(OIDCClientConfig{RegistryURL: "https://registry.npmjs.org/"})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("invalid package name", func(t *testing.T) {
		t.Parallel()
		result := client.Exchange(t.Context(), "foo bar", newSecretToken(testIdentityToken))
		assertReportPrimary(t, result.Report, IDNPMOIDCExchangeIndeterminate)
	})

	t.Run("empty identity token", func(t *testing.T) {
		t.Parallel()
		result := client.Exchange(t.Context(), "safe-package", newSecretToken(""))
		assertReportPrimary(t, result.Report, IDNPMOIDCExchangeIndeterminate)
	})
}

// TestDecodeGitHubOIDCResponseRejections pins the strict GitHub identity
// response grammar: an empty value, trailing data, undecodable claims, a
// claim without a ref separator, and a workflow path outside
// .github/workflows all fail closed.
func TestDecodeGitHubOIDCResponseRejections(t *testing.T) {
	t.Parallel()
	jwtWithClaims := func(claims string) string {
		return "header." + base64.RawURLEncoding.EncodeToString([]byte(claims)) + ".signature"
	}
	tests := []struct {
		name    string
		body    string
		ref     string
		wantErr string
	}{
		{name: "value missing", body: `{}`, ref: "ref", wantErr: "invalid GitHub OIDC response"},
		{name: "claims not decodable", body: `{"value":"header.####.signature"}`, ref: "ref", wantErr: "invalid GitHub OIDC claims"},
		{
			name:    "workflow claim has no ref",
			body:    fmt.Sprintf(`{"value":%q}`, jwtWithClaims(`{"workflow_ref":"norefsign"}`)),
			ref:     "norefsign",
			wantErr: "caller workflow claim has no ref",
		},
		{
			name:    "workflow path outside .github/workflows",
			body:    fmt.Sprintf(`{"value":%q}`, jwtWithClaims(`{"workflow_ref":"windlasstech/caller/build.yml@refs/tags/v1.2.3"}`)),
			ref:     "windlasstech/caller/build.yml@refs/tags/v1.2.3",
			wantErr: "caller workflow path is invalid",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			token, filename, err := decodeGitHubOIDCResponse([]byte(test.body), test.ref)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("decodeGitHubOIDCResponse() error = %v, want %q", err, test.wantErr)
			}
			if token.valid() || filename != "" {
				t.Fatalf("decodeGitHubOIDCResponse() = (%v, %q) on rejection, want zero values", token, filename)
			}
		})
	}
}
