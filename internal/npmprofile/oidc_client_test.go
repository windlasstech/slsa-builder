package npmprofile

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/windlasstech/slsa-builder/internal/diagnostic"
)

const (
	testIdentityToken = "github-oidc-secret-value"
	testPublishToken  = "npm-short-lived-secret-value"
)

var testExchangeNow = time.Date(2026, 8, 7, 12, 0, 1, 0, time.UTC)

func TestOIDCExchangeSuccess(t *testing.T) {
	t.Parallel()

	server := newOIDCExchangeServer(t, http.StatusCreated, fmt.Sprintf(
		`{"token_type":"oidc","token":%q,"created":"2026-08-07T12:00:00Z","expires":"2026-08-07T13:00:00Z"}`,
		testPublishToken,
	))
	defer server.Close()

	client := newTestOIDCClient(t, server)
	result := client.Exchange(context.Background(), "@windlass/slsa-builder", newSecretToken(testIdentityToken))
	assertOIDCPass(t, result)
	if result.Token.value() != testPublishToken {
		t.Fatal("Exchange() did not preserve the exchanged token in memory")
	}
	if !result.ExpiresAt.Equal(time.Date(2026, 8, 7, 13, 0, 0, 0, time.UTC)) {
		t.Fatalf("expires_at = %s", result.ExpiresAt)
	}
}

// ADR 0081: the live registry returns created/expires as epoch-second JSON numbers, while npm's
// documentation example shows RFC 3339 strings. Both representations must be accepted.
func TestOIDCExchangeSuccessEpochSeconds(t *testing.T) {
	t.Parallel()

	created := testExchangeNow.Add(-time.Second).Unix()
	expires := testExchangeNow.Add(15 * time.Minute).Unix()
	server := newOIDCExchangeServer(t, http.StatusCreated, fmt.Sprintf(
		`{"token_type":"oidc","token":%q,"created":%d,"expires":%d}`,
		testPublishToken, created, expires,
	))
	defer server.Close()

	result := newTestOIDCClient(t, server).Exchange(context.Background(), "@windlass/slsa-builder", newSecretToken(testIdentityToken))
	assertOIDCPass(t, result)
	if result.Token.value() != testPublishToken {
		t.Fatal("Exchange() did not preserve the exchanged token in memory")
	}
	if !result.CreatedAt.Equal(time.Unix(created, 0).UTC()) {
		t.Fatalf("created_at = %s", result.CreatedAt)
	}
	if !result.ExpiresAt.Equal(time.Unix(expires, 0).UTC()) {
		t.Fatalf("expires_at = %s", result.ExpiresAt)
	}
}

// ADR 0081: every other strictness rule is retained; each contract violation must fail before
// registry mutation with npm-oidc-exchange-indeterminate.
func TestOIDCExchangeResponseContractRejections(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"unknown member":        `{"token_type":"oidc","token":"x","created":"2026-08-07T12:00:00Z","expires":"2026-08-07T12:15:00Z","scope":"publish"}`,
		"missing member":        `{"token_type":"oidc","token":"x","created":"2026-08-07T12:00:00Z"}`,
		"wrong token_type":      `{"token_type":"bearer","token":"x","created":"2026-08-07T12:00:00Z","expires":"2026-08-07T12:15:00Z"}`,
		"inverted lifetime":     `{"token_type":"oidc","token":"x","created":"2026-08-07T12:15:00Z","expires":"2026-08-07T12:00:00Z"}`,
		"expired at exchange":   `{"token_type":"oidc","token":"x","created":"2026-08-07T11:45:00Z","expires":"2026-08-07T12:00:00Z"}`,
		"malformed string":      `{"token_type":"oidc","token":"x","created":"not-a-timestamp","expires":"2026-08-07T12:15:00Z"}`,
		"fractional epoch":      `{"token_type":"oidc","token":"x","created":1786705013,"expires":1786705913.5}`,
		"non-positive epoch":    `{"token_type":"oidc","token":"x","created":1786705013,"expires":0}`,
		"epoch as bare string":  `{"token_type":"oidc","token":"x","created":"1786705013","expires":"2026-08-07T12:15:00Z"}`,
		"mixed representations": `{"token_type":"oidc","token":"x","created":1786705013,"expires":"not-a-timestamp"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			assertOIDCExchangeFailure(t, http.StatusCreated, body, IDNPMOIDCExchangeIndeterminate)
		})
	}
}

// ADR 0081: a success body mixing the observed epoch-number and documented RFC 3339
// representations across created/expires is accepted and normalized to instants.
func TestOIDCExchangeSuccessMixedTimestamps(t *testing.T) {
	t.Parallel()

	created := testExchangeNow.Add(-time.Second).Unix()
	server := newOIDCExchangeServer(t, http.StatusCreated, fmt.Sprintf(
		`{"token_type":"oidc","token":%q,"created":%d,"expires":"2026-08-07T12:15:00Z"}`,
		testPublishToken, created,
	))
	defer server.Close()

	result := newTestOIDCClient(t, server).Exchange(context.Background(), "@windlass/slsa-builder", newSecretToken(testIdentityToken))
	assertOIDCPass(t, result)
	if !result.CreatedAt.Equal(time.Unix(created, 0).UTC()) {
		t.Fatalf("created_at = %s", result.CreatedAt)
	}
	if !result.ExpiresAt.Equal(time.Date(2026, 8, 7, 12, 15, 0, 0, time.UTC)) {
		t.Fatalf("expires_at = %s", result.ExpiresAt)
	}
}

func TestOIDCExchange401(t *testing.T) {
	t.Parallel()
	assertOIDCExchangeFailure(t, http.StatusUnauthorized, `{"error":"unauthorized"}`, IDTrustedPublisherMismatch)
}

func TestOIDCExchange404(t *testing.T) {
	t.Parallel()
	assertOIDCExchangeFailure(t, http.StatusNotFound, `{"error":"not found"}`, IDTrustedPublisherMismatch)
}

func TestOIDCExchange5XX(t *testing.T) {
	t.Parallel()
	assertOIDCExchangeFailure(t, http.StatusServiceUnavailable, `{"error":"temporary"}`, IDNPMOIDCExchangeIndeterminate)
}

func TestOIDCExchangeMalformed(t *testing.T) {
	t.Parallel()
	assertOIDCExchangeFailure(t, http.StatusCreated, `{"token_type":"oidc","token":`, IDNPMOIDCExchangeIndeterminate)
}

func TestTokenRedaction(t *testing.T) {
	t.Parallel()

	server := newOIDCExchangeServer(t, http.StatusCreated,
		`{"token_type":"oidc","token":"`+testPublishToken+`","created":"2026-08-07T12:00:00Z","expires":"2026-08-07T13:00:00Z"}`)
	defer server.Close()
	result := newTestOIDCClient(t, server).Exchange(context.Background(), "safe-package", newSecretToken(testIdentityToken))
	assertOIDCPass(t, result)

	for _, rendered := range []string{
		fmt.Sprint(result.Token),
		fmt.Sprintf("%v", result.Token),
		fmt.Sprintf("%+v", result.Token),
		fmt.Sprintf("%#v", result.Token),
		result.Token.String(),
	} {
		if strings.Contains(rendered, testIdentityToken) || strings.Contains(rendered, testPublishToken) {
			t.Fatalf("token rendering leaked secret material: %q", rendered)
		}
	}
	report, err := result.Report.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(report), testIdentityToken) || strings.Contains(string(report), testPublishToken) {
		t.Fatalf("diagnostic report leaked token material: %s", report)
	}
}

func TestOIDCCapabilityAndCallerWorkflow(t *testing.T) {
	t.Parallel()

	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"workflow_ref":"windlasstech/caller/.github/workflows/release.yml@refs/tags/v1.2.3"}`))
	jwt := "header." + payload + ".signature"
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Query().Get("audience") != npmOIDCAudience {
			t.Errorf("OIDC request = %s %s", request.Method, request.URL.String())
		}
		if request.Header.Get("Authorization") != "Bearer "+testIdentityToken {
			t.Error("OIDC request authorization was not the capability token")
		}
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(writer, `{"value":%q}`, jwt)
	}))
	defer server.Close()

	client, err := NewOIDCClient(OIDCClientConfig{
		HTTPClient:          server.Client(),
		RegistryURL:         "https://registry.npmjs.org/",
		IDTokenRequestURL:   server.URL + "?request=opaque",
		IDTokenRequestToken: testIdentityToken,
		GitHubWorkflowRef:   "windlasstech/caller/.github/workflows/release.yml@refs/tags/v1.2.3",
	})
	if err != nil {
		t.Fatal(err)
	}
	identity, report := client.AcquireIdentity(context.Background())
	assertReportPrimary(t, report, "")
	if identity.WorkflowFilename != "release.yml" || identity.Token.value() != jwt {
		t.Fatalf("identity = %#v", identity)
	}
}

func TestOIDCCapabilityUnavailable(t *testing.T) {
	t.Parallel()

	client, err := NewOIDCClient(OIDCClientConfig{RegistryURL: "https://registry.npmjs.org/"})
	if err != nil {
		t.Fatal(err)
	}
	_, report := client.AcquireIdentity(context.Background())
	assertReportPrimary(t, report, IDOIDCCapabilityUnavailable)
}

func TestOIDCRejectsRedirect(t *testing.T) {
	t.Parallel()

	target := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("credential-bearing redirect target was reached")
	}))
	defer target.Close()
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL, http.StatusTemporaryRedirect)
	}))
	defer server.Close()

	result := newTestOIDCClient(t, server).Exchange(context.Background(), "safe-package", newSecretToken(testIdentityToken))
	assertReportPrimary(t, result.Report, IDNPMOIDCExchangeIndeterminate)
}

func TestOIDCRejectsCredentialBearingProxy(t *testing.T) {
	t.Parallel()

	var requests atomic.Int64
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer server.Close()
	serverClient := server.Client()
	baseTransport, ok := serverClient.Transport.(*http.Transport)
	if !ok {
		t.Fatal("test server client has an unexpected transport")
	}
	transport := baseTransport.Clone()
	transport.Proxy = func(*http.Request) (*url.URL, error) {
		return url.Parse("https://user:password@proxy.example")
	}
	serverClient.Transport = transport
	client, err := NewOIDCClient(OIDCClientConfig{HTTPClient: serverClient, RegistryURL: server.URL + "/"})
	if err != nil {
		t.Fatal(err)
	}
	result := client.Exchange(context.Background(), "safe-package", newSecretToken(testIdentityToken))
	assertReportPrimary(t, result.Report, IDNPMOIDCExchangeIndeterminate)
	if requests.Load() != 0 {
		t.Fatal("request reached the network through a credential-bearing proxy")
	}
}

func TestOIDCResponseSizeLimit(t *testing.T) {
	t.Parallel()

	server := newOIDCExchangeServer(t, http.StatusCreated, strings.Repeat("x", int(maxOIDCResponse)+1))
	defer server.Close()
	result := newTestOIDCClient(t, server).Exchange(context.Background(), "safe-package", newSecretToken(testIdentityToken))
	assertReportPrimary(t, result.Report, IDNPMOIDCExchangeIndeterminate)
}

func assertOIDCExchangeFailure(t *testing.T, status int, body, wantID string) {
	t.Helper()
	server := newOIDCExchangeServer(t, status, body)
	defer server.Close()
	result := newTestOIDCClient(t, server).Exchange(context.Background(), "@windlass/slsa-builder", newSecretToken(testIdentityToken))
	assertReportPrimary(t, result.Report, wantID)
	if result.Token.valid() {
		t.Fatal("failed exchange retained a publish token")
	}
}

func newOIDCExchangeServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", request.Method)
		}
		if request.URL.EscapedPath() != "/-/npm/v1/oidc/token/exchange/package/%40windlass%2Fslsa-builder" &&
			request.URL.EscapedPath() != "/-/npm/v1/oidc/token/exchange/package/safe-package" {
			t.Errorf("exchange path = %q", request.URL.EscapedPath())
		}
		if request.Header.Get("Authorization") != "Bearer "+testIdentityToken {
			t.Error("exchange request did not use the identity token")
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(status)
		if _, err := writer.Write([]byte(body)); err != nil {
			t.Errorf("write exchange response: %v", err)
		}
	}))
}

func newTestOIDCClient(t *testing.T, server *httptest.Server) *OIDCClient {
	t.Helper()
	client, err := NewOIDCClient(OIDCClientConfig{
		HTTPClient:  server.Client(),
		RegistryURL: server.URL + "/",
		Now: func() time.Time {
			return testExchangeNow
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func assertOIDCPass(t *testing.T, result OIDCExchangeResult) {
	t.Helper()
	assertReportPrimary(t, result.Report, "")
	if !result.Token.valid() {
		t.Fatal("successful exchange has no publish token")
	}
}

func assertReportPrimary(t *testing.T, report diagnostic.Report, want string) {
	t.Helper()
	if want == "" {
		if report.PrimaryID != nil || report.ExitCode != 0 {
			t.Fatalf("report = %#v, want pass", report)
		}
		return
	}
	if report.PrimaryID == nil || *report.PrimaryID != want {
		t.Fatalf("primary ID = %v, want %q", report.PrimaryID, want)
	}
}
