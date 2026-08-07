// Package signing creates keyless Sigstore DSSE bundles in GitHub Actions.
package signing

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/sign"
	"github.com/windlasstech/slsa-builder/internal/attestation"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

const (
	fulcioURL    = "https://fulcio.sigstore.dev"
	rekorURL     = "https://rekor.sigstore.dev"
	oidcAudience = "sigstore"
)

const networkTimeout = 90 * time.Second

// Request is the closed input to the GitHub Actions keyless signing boundary.
type Request struct {
	Statement []byte
	Identity  attestation.IdentityExpectation
}

// Result preserves the exact Statement and serialized Sigstore bundle bytes.
type Result struct {
	Statement []byte
	Bundle    []byte
}

// PrepareDSSE validates but never normalizes the exact preassembled Statement bytes.
func PrepareDSSE(statement []byte) (sign.DSSEData, error) {
	decoded, err := provenance.DecodeStatement(statement)
	if err != nil {
		return sign.DSSEData{}, fmt.Errorf("decode exact Statement: %w", err)
	}
	canonical, err := decoded.CanonicalJSON()
	if err != nil {
		return sign.DSSEData{}, fmt.Errorf("canonicalize exact Statement: %w", err)
	}
	if !bytes.Equal(statement, canonical) {
		return sign.DSSEData{}, errors.New("statement must be exact RFC 8785 bytes")
	}
	return sign.DSSEData{Data: append([]byte(nil), statement...), PayloadType: bundle.IntotoMediaType}, nil
}

// SignGitHubActions performs the supported sigstore-go keyless signing flow.
func SignGitHubActions(ctx context.Context, request Request) (Result, error) {
	if ctx == nil {
		return Result{}, errors.New("signing context is required")
	}
	content, err := PrepareDSSE(request.Statement)
	if err != nil {
		return Result{}, err
	}
	trustedRoot, err := root.FetchTrustedRoot()
	if err != nil {
		return Result{}, fmt.Errorf("fetch Sigstore trusted root: %w", err)
	}
	token, err := githubActionsOIDCToken(ctx)
	if err != nil {
		return Result{}, err
	}
	identity, err := bindOIDCSignerIdentity(content.Data, request.Identity, token)
	if err != nil {
		return Result{}, err
	}
	keypair, err := sign.NewEphemeralKeypair(nil)
	if err != nil {
		return Result{}, fmt.Errorf("create ephemeral signing key: %w", err)
	}
	fulcio := sign.NewFulcio(&sign.FulcioOptions{BaseURL: fulcioURL, Timeout: networkTimeout, Retries: 1})
	rekor := sign.NewRekor(&sign.RekorOptions{BaseURL: rekorURL, Timeout: networkTimeout, Retries: 1, Version: 1})
	signed, err := sign.Bundle(&content, keypair, sign.BundleOptions{
		CertificateProvider:        fulcio,
		CertificateProviderOptions: &sign.CertificateProviderOptions{IDToken: token},
		TransparencyLogs:           []sign.Transparency{rekor},
		TrustedRoot:                trustedRoot,
	})
	if err != nil {
		return Result{}, fmt.Errorf("create Sigstore DSSE bundle: %w", err)
	}
	parsedBundle, err := bundle.NewBundle(signed)
	if err != nil {
		return Result{}, fmt.Errorf("load generated Sigstore DSSE bundle: %w", err)
	}
	serialized, err := parsedBundle.MarshalJSON()
	if err != nil {
		return Result{}, fmt.Errorf("serialize Sigstore DSSE bundle: %w", err)
	}
	verified, err := attestation.VerifyWithTrustedMaterial(ctx, attestation.Request{
		Bundle:                serialized,
		Identity:              identity,
		ExpectedStatementJSON: content.Data,
	}, trustedRoot)
	if err != nil {
		return Result{}, fmt.Errorf("post-sign offline verification: %w", err)
	}
	if !bytes.Equal(verified.BundleBytes(), serialized) || !bytes.Equal(verified.StatementBytes(), content.Data) {
		return Result{}, errors.New("post-sign verification did not preserve exact bundle and Statement bytes")
	}
	return Result{Statement: append([]byte(nil), content.Data...), Bundle: serialized}, nil
}

func bindOIDCSignerIdentity(statementBytes []byte, identity attestation.IdentityExpectation, token string) (attestation.IdentityExpectation, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return attestation.IdentityExpectation{}, errors.New("GitHub Actions OIDC token shape is invalid")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return attestation.IdentityExpectation{}, errors.New("GitHub Actions OIDC claims are invalid")
	}
	var claims struct {
		JobWorkflowRef string `json:"job_workflow_ref"`
		JobWorkflowSHA string `json:"job_workflow_sha"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.JobWorkflowRef == "" || claims.JobWorkflowSHA == "" {
		return attestation.IdentityExpectation{}, errors.New("GitHub Actions OIDC workflow claims are unavailable")
	}
	if claims.JobWorkflowSHA != identity.WorkflowSHA {
		return attestation.IdentityExpectation{}, errors.New("GitHub Actions OIDC workflow revision differs from trusted runtime")
	}
	signerURI := "https://github.com/" + claims.JobWorkflowRef
	statement, err := provenance.DecodeStatement(statementBytes)
	if err != nil {
		return attestation.IdentityExpectation{}, fmt.Errorf("decode signed Statement identity: %w", err)
	}
	builderPath, _, builderFound := strings.Cut(statement.Predicate.RunDetails.Builder.ID, "@")
	signerPath, _, signerFound := strings.Cut(signerURI, "@")
	if !builderFound || !signerFound || builderPath != signerPath {
		return attestation.IdentityExpectation{}, errors.New("OIDC signer workflow differs from the Statement builder identity")
	}
	identity.SignerURI = signerURI
	return identity, nil
}

func githubActionsOIDCToken(ctx context.Context) (string, error) {
	requestURL := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_URL")
	requestToken := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")
	if requestURL == "" || requestToken == "" {
		return "", errors.New("GitHub Actions OIDC capability is unavailable")
	}
	parsed, err := url.Parse(requestURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return "", errors.New("GitHub Actions OIDC request URL is invalid")
	}
	query := parsed.Query()
	query.Set("audience", oidcAudience)
	parsed.RawQuery = query.Encode()
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return "", fmt.Errorf("construct GitHub Actions OIDC request: %w", err)
	}
	httpRequest.Header.Set("Authorization", "Bearer "+requestToken)
	client := &http.Client{
		Timeout: networkTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errors.New("GitHub Actions OIDC redirects are forbidden")
		},
	}
	response, err := client.Do(httpRequest)
	if err != nil {
		return "", fmt.Errorf("request GitHub Actions OIDC token: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request GitHub Actions OIDC token: unexpected HTTP status %d", response.StatusCode)
	}
	var document struct {
		Value string `json:"value"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil || strings.TrimSpace(document.Value) == "" {
		return "", errors.New("GitHub Actions OIDC response is invalid")
	}
	return document.Value, nil
}
