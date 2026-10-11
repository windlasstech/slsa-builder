package npmprofile

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestPreflightOIDC(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name                               string
		capability                         bool
		identityStatus, exchangeStatus     int
		wantID, wantWorkflow               string
		identityRequests, exchangeRequests int64
	}{
		{"capability missing", false, http.StatusOK, http.StatusCreated, IDOIDCCapabilityUnavailable, "", 0, 0},
		{"identity unavailable", true, http.StatusBadRequest, http.StatusCreated, IDOIDCCapabilityUnavailable, "", 1, 0},
		{"exchange denied", true, http.StatusOK, http.StatusUnauthorized, IDTrustedPublisherMismatch, "release.yml", 1, 1},
		{"exchange indeterminate", true, http.StatusOK, http.StatusBadRequest, IDNPMOIDCExchangeIndeterminate, "release.yml", 1, 1},
		{"success", true, http.StatusOK, http.StatusCreated, "", "release.yml", 1, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			workflowRef := "windlasstech/caller/.github/workflows/release.yml@refs/tags/v1.2.3"
			payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"workflow_ref":%q}`, workflowRef)))
			jwt := "header." + payload + ".signature"
			var identityRequests, exchangeRequests atomic.Int64
			server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				writer.Header().Set("Content-Type", "application/json")
				switch request.URL.EscapedPath() {
				case "/identity":
					identityRequests.Add(1)
					if request.Method != http.MethodGet || request.URL.Query().Get("audience") != "npm:"+request.Host {
						t.Errorf("identity request = %s %s, want GET with registry audience", request.Method, request.URL)
					}
					if request.Header.Get("Authorization") != "Bearer "+testIdentityToken {
						t.Error("identity request did not use capability token")
					}
					writer.WriteHeader(test.identityStatus)
					if _, err := fmt.Fprintf(writer, `{"value":%q}`, jwt); err != nil {
						t.Errorf("write identity response: %v", err)
					}
				case "/-/npm/v1/oidc/token/exchange/package/%40windlass%2Fslsa-builder":
					exchangeRequests.Add(1)
					if request.Method != http.MethodPost {
						t.Errorf("exchange method = %s, want POST", request.Method)
					}
					if request.Header.Get("Authorization") != "Bearer "+jwt {
						t.Error("exchange did not use acquired identity token")
					}
					writer.WriteHeader(test.exchangeStatus)
					if _, err := fmt.Fprintf(writer, `{"token_type":"oidc","token":%q,"created":"2026-08-07T12:00:00Z","expires":"2026-08-07T12:15:00Z"}`, testPublishToken); err != nil {
						t.Errorf("write exchange response: %v", err)
					}
				default:
					t.Errorf("unexpected preflight path %q", request.URL.EscapedPath())
					writer.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			config := OIDCClientConfig{HTTPClient: server.Client(), RegistryURL: server.URL + "/", GitHubWorkflowRef: workflowRef, Now: func() time.Time { return testExchangeNow }}
			if test.capability {
				config.IDTokenRequestURL, config.IDTokenRequestToken = server.URL+"/identity", testIdentityToken
			}
			client, err := NewOIDCClient(config)
			if err != nil {
				t.Fatal(err)
			}
			result := client.Preflight(t.Context(), "@windlass/slsa-builder")
			assertReportPrimary(t, result.Report, test.wantID)
			if result.WorkflowFilename != test.wantWorkflow {
				t.Fatalf("workflow filename = %q, want %q", result.WorkflowFilename, test.wantWorkflow)
			}
			if identityRequests.Load() != test.identityRequests || exchangeRequests.Load() != test.exchangeRequests {
				t.Fatalf("requests identity/exchange = %d/%d, want %d/%d", identityRequests.Load(), exchangeRequests.Load(), test.identityRequests, test.exchangeRequests)
			}
			if test.wantID != "" {
				if result.Token.valid() || !result.CreatedAt.IsZero() || !result.ExpiresAt.IsZero() {
					t.Fatal("failed preflight retained exchange credentials or timestamps")
				}
				return
			}
			if result.Token.value() != testPublishToken || !result.CreatedAt.Equal(testExchangeNow.Add(-time.Second)) || !result.ExpiresAt.Equal(time.Date(2026, 8, 7, 12, 15, 0, 0, time.UTC)) {
				t.Fatal("successful preflight did not preserve exchange token and lifetime")
			}
		})
	}
}
