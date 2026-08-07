package npmprofile

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
	"github.com/windlasstech/slsa-builder/internal/diagnostic"
)

const (
	IDTrustedPublisherMismatch           = "windlass.verify.error.trusted-publisher-mismatch"
	IDNPMOIDCExchangeIndeterminate       = "windlass.verify.error.npm-oidc-exchange-indeterminate"
	IDOIDCCapabilityUnavailable          = "windlass.verify.error.oidc-capability-unavailable"
	npmOIDCAudience                      = "npm:registry.npmjs.org"
	maxOIDCResponse                int64 = 64 << 10
)

// OIDCClientConfig supplies the GitHub capability observation and npm registry endpoint.
type OIDCClientConfig struct {
	HTTPClient          *http.Client
	RegistryURL         string
	IDTokenRequestURL   string
	IDTokenRequestToken string
	GitHubWorkflowRef   string
	Now                 func() time.Time
}

// OIDCClient acquires a GitHub identity token and exchanges it for one short-lived npm token.
type OIDCClient struct {
	httpClient          *http.Client
	registryURL         *url.URL
	idTokenRequestURL   string
	idTokenRequestToken SecretToken
	githubWorkflowRef   string
	audience            string
	now                 func() time.Time
}

// OIDCIdentity is the secret GitHub JWT plus the observed caller workflow filename.
type OIDCIdentity struct {
	Token            SecretToken
	WorkflowFilename string
}

// OIDCExchangeResult is the classified result of the non-mutating npm token exchange.
type OIDCExchangeResult struct {
	Token            SecretToken
	CreatedAt        time.Time
	ExpiresAt        time.Time
	WorkflowFilename string
	Report           diagnostic.Report
}

// NewOIDCClient validates endpoints and installs TLS, redirect, timeout, and proxy controls.
func NewOIDCClient(config OIDCClientConfig) (*OIDCClient, error) {
	registryURL, err := normalizeRegistryURL(config.RegistryURL)
	if err != nil {
		return nil, err
	}
	client, err := hardenedHTTPClient(config.HTTPClient)
	if err != nil {
		return nil, err
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &OIDCClient{
		httpClient:          client,
		registryURL:         registryURL,
		idTokenRequestURL:   config.IDTokenRequestURL,
		idTokenRequestToken: newSecretToken(config.IDTokenRequestToken),
		githubWorkflowRef:   config.GitHubWorkflowRef,
		audience:            "npm:" + registryURL.Host,
		now:                 now,
	}, nil
}

// AcquireIdentity performs the side-effect-free GitHub Actions OIDC capability observation.
func (client *OIDCClient) AcquireIdentity(ctx context.Context) (OIDCIdentity, diagnostic.Report) {
	if client.idTokenRequestURL == "" || !client.idTokenRequestToken.valid() {
		return OIDCIdentity{}, oidcFailureReport(
			IDOIDCCapabilityUnavailable,
			"oidc.capability",
			"GitHub Actions OIDC capability is unavailable",
		)
	}
	endpoint, err := url.Parse(client.idTokenRequestURL)
	if err != nil || endpoint.Scheme != "https" || endpoint.Host == "" || endpoint.User != nil || endpoint.Fragment != "" {
		return OIDCIdentity{}, oidcFailureReport(
			IDOIDCCapabilityUnavailable,
			"oidc.capability",
			"GitHub Actions OIDC request endpoint is unusable",
		)
	}
	query := endpoint.Query()
	query.Set("audience", client.audience)
	endpoint.RawQuery = query.Encode()
	headers := make(http.Header)
	headers.Set("Accept", "application/json; api-version=2.0")
	headers.Set("Authorization", "Bearer "+client.idTokenRequestToken.value())
	headers.Set("Content-Type", "application/json")
	headers.Set("User-Agent", "windlass-slsa-builder")
	response, err := performBoundedRequest(ctx, client.httpClient, http.MethodGet, endpoint, headers, maxOIDCResponse)
	if err != nil || response.status != http.StatusOK {
		return OIDCIdentity{}, oidcFailureReport(
			IDOIDCCapabilityUnavailable,
			"oidc.capability",
			"GitHub Actions could not issue an OIDC identity token",
		)
	}
	identityToken, workflowFilename, err := decodeGitHubOIDCResponse(response.body, client.githubWorkflowRef)
	if err != nil {
		return OIDCIdentity{}, oidcFailureReport(
			IDTrustedPublisherMismatch,
			"oidc.caller-workflow",
			"caller workflow identity is unavailable or conflicts with GitHub context",
		)
	}
	return OIDCIdentity{Token: identityToken, WorkflowFilename: workflowFilename}, oidcPassReport()
}

// Preflight acquires GitHub identity and performs the early npm exchange before signing or publish.
func (client *OIDCClient) Preflight(ctx context.Context, packageName string) OIDCExchangeResult {
	identity, report := client.AcquireIdentity(ctx)
	if report.PrimaryID != nil {
		return OIDCExchangeResult{Report: report}
	}
	result := client.Exchange(ctx, packageName, identity.Token)
	result.WorkflowFilename = identity.WorkflowFilename
	return result
}

// Exchange performs npm's non-mutating trusted-publisher exchange and exact status classification.
func (client *OIDCClient) Exchange(ctx context.Context, packageName string, identityToken SecretToken) OIDCExchangeResult {
	if !validRegistryPackageName(packageName) || !identityToken.valid() {
		return OIDCExchangeResult{Report: oidcFailureReport(
			IDNPMOIDCExchangeIndeterminate,
			"npm.oidc-exchange",
			"npm OIDC exchange inputs are unusable",
		)}
	}
	endpoint := *client.registryURL
	endpoint.Path = "/-/npm/v1/oidc/token/exchange/package/" + packageName
	endpoint.RawPath = "/-/npm/v1/oidc/token/exchange/package/" + percentEncode(packageName)
	headers := make(http.Header)
	headers.Set("Accept", "application/json")
	headers.Set("Authorization", "Bearer "+identityToken.value())
	headers.Set("User-Agent", "windlass-slsa-builder")
	response, err := performBoundedRequest(ctx, client.httpClient, http.MethodPost, &endpoint, headers, maxOIDCResponse)
	if err != nil {
		return OIDCExchangeResult{Report: oidcFailureReport(
			IDNPMOIDCExchangeIndeterminate,
			"npm.oidc-exchange",
			"npm OIDC exchange could not establish trusted-publisher state",
		)}
	}
	if response.status == http.StatusUnauthorized || response.status == http.StatusNotFound {
		return OIDCExchangeResult{Report: oidcFailureReport(
			IDTrustedPublisherMismatch,
			"npm.oidc-exchange",
			"npm trusted-publisher configuration does not authorize this caller workflow",
		)}
	}
	if response.status != http.StatusCreated {
		return OIDCExchangeResult{Report: oidcFailureReport(
			IDNPMOIDCExchangeIndeterminate,
			"npm.oidc-exchange",
			"npm OIDC exchange returned an unclassifiable response",
		)}
	}
	publishToken, createdAt, expiresAt, err := decodeNPMExchangeResponse(response.body, client.now())
	if err != nil {
		return OIDCExchangeResult{Report: oidcFailureReport(
			IDNPMOIDCExchangeIndeterminate,
			"npm.oidc-exchange",
			"npm OIDC exchange returned a malformed response",
		)}
	}
	return OIDCExchangeResult{
		Token:     publishToken,
		CreatedAt: createdAt,
		ExpiresAt: expiresAt,
		Report:    oidcPassReport(),
	}
}

func decodeGitHubOIDCResponse(encoded []byte, githubWorkflowRef string) (SecretToken, string, error) {
	if err := canonicaljson.Validate(encoded); err != nil {
		return SecretToken{}, "", err
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var response struct {
		Value string `json:"value"`
	}
	if err := decoder.Decode(&response); err != nil || response.Value == "" {
		return SecretToken{}, "", errors.New("invalid GitHub OIDC response")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return SecretToken{}, "", errors.New("invalid GitHub OIDC response")
	}
	parts := strings.Split(response.Value, ".")
	if len(parts) != 3 {
		return SecretToken{}, "", errors.New("invalid GitHub OIDC token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || canonicaljson.Validate(payload) != nil {
		return SecretToken{}, "", errors.New("invalid GitHub OIDC claims")
	}
	var claims struct {
		WorkflowRef string `json:"workflow_ref"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.WorkflowRef == "" || claims.WorkflowRef != githubWorkflowRef {
		return SecretToken{}, "", errors.New("caller workflow claim mismatch")
	}
	workflowPath, _, found := strings.Cut(claims.WorkflowRef, "@")
	if !found {
		return SecretToken{}, "", errors.New("caller workflow claim has no ref")
	}
	filename := path.Base(workflowPath)
	if filename == "." || filename == "/" || !strings.Contains(workflowPath, "/.github/workflows/") {
		return SecretToken{}, "", errors.New("caller workflow path is invalid")
	}
	return newSecretToken(response.Value), filename, nil
}

func decodeNPMExchangeResponse(encoded []byte, now time.Time) (SecretToken, time.Time, time.Time, error) {
	if err := canonicaljson.Validate(encoded); err != nil {
		return SecretToken{}, time.Time{}, time.Time{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var response struct {
		TokenType string `json:"token_type"`
		Token     string `json:"token"`
		Created   string `json:"created"`
		Expires   string `json:"expires"`
	}
	if err := decoder.Decode(&response); err != nil {
		return SecretToken{}, time.Time{}, time.Time{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return SecretToken{}, time.Time{}, time.Time{}, errors.New("multiple JSON values")
	}
	createdAt, createdErr := time.Parse(time.RFC3339, response.Created)
	expiresAt, expiresErr := time.Parse(time.RFC3339, response.Expires)
	if response.TokenType != "oidc" || response.Token == "" || strings.ContainsAny(response.Token, "\r\n\x00") ||
		createdErr != nil || expiresErr != nil || !expiresAt.After(createdAt) || !expiresAt.After(now) {
		return SecretToken{}, time.Time{}, time.Time{}, errors.New("invalid npm OIDC exchange response")
	}
	return newSecretToken(response.Token), createdAt, expiresAt, nil
}

func oidcPassReport() diagnostic.Report {
	report, err := diagnostic.Build(nil, nil, nil)
	if err != nil {
		panic(err)
	}
	return report
}

func oidcFailureReport(id, check, message string) diagnostic.Report {
	entry, err := diagnostic.New(id, check, message)
	if err != nil {
		panic(err)
	}
	report, err := diagnostic.Build(nil, []diagnostic.Diagnostic{entry}, nil)
	if err != nil {
		panic(err)
	}
	return report
}
