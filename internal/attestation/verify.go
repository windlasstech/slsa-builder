package attestation

import (
	"context"
	"crypto/x509"
	"encoding/asn1"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/verify"
	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
	"github.com/windlasstech/slsa-builder/internal/identity"
	"github.com/windlasstech/slsa-builder/internal/policy"
)

const (
	publicGoodInstance    = "sigstore-public-good"
	publicGoodTUF         = "https://tuf-repo-cdn.sigstore.dev"
	policyTimestampLayout = "2006-01-02T15:04:05Z"
)

var forbiddenTrustRootOverrides = []string{
	"SIGSTORE_ROOT_FILE",
	"SIGSTORE_REKOR_PUBLIC_KEY",
	"SIGSTORE_CT_LOG_PUBLIC_KEY_FILE",
}

var positiveDecimalPattern = regexp.MustCompile(`^[1-9][0-9]*$`)

var (
	oidIssuerV2            = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 8}
	oidBuildSignerURI      = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 9}
	oidBuildSignerDigest   = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 10}
	oidRunnerEnvironment   = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 11}
	oidSourceRepositoryURI = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 12}
	oidSourceDigest        = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 13}
	oidSourceRef           = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 14}
	oidSourceRepositoryID  = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 15}
	oidSourceOwnerID       = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 17}
	oidRunInvocationURI    = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 21}
)

// Verify authenticates the bundle, transparency evidence, signing time, signer, and expected Statement.
func Verify(ctx context.Context, request Request) (Result, error) {
	return verifyAt(ctx, request, time.Now().UTC())
}

// VerifyWithTrustedMaterial performs network-free verification with already authenticated material.
func VerifyWithTrustedMaterial(ctx context.Context, request Request, trustedMaterial root.TrustedMaterial) (Result, error) {
	return verifyWithTrustedMaterial(ctx, request, trustedMaterial)
}

func verifyAt(ctx context.Context, request Request, verificationTime time.Time) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, newError(idVerifierExecutionFailure, "context", "verification context is not usable", err)
	}
	hasModel := request.ExpectedStatement != nil
	hasJSON := len(request.ExpectedStatementJSON) > 0
	if !hasModel && !hasJSON {
		return Result{}, newError(idInputUnavailable, "expected_statement", "an expected Statement is required", nil)
	}
	if hasModel && hasJSON {
		return Result{}, newError(idPolicySchemaInvalid, "expected_statement", "exactly one expected Statement representation is allowed", nil)
	}
	if err := validateIdentityExpectation(request.Identity); err != nil {
		return Result{}, err
	}
	parsed, err := ParseBundle(request.Bundle)
	if err != nil {
		return Result{}, err
	}
	if err := rejectLegacyOverrides(); err != nil {
		return Result{}, err
	}
	trustedMaterial, err := acquireTrustedMaterial(request, verificationTime)
	if err != nil {
		return Result{}, err
	}
	return verifyParsedWithTrustedMaterial(request, parsed, trustedMaterial)
}

func verifyWithTrustedMaterial(ctx context.Context, request Request, trustedMaterial root.TrustedMaterial) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, newError(idVerifierExecutionFailure, "context", "verification context is not usable", err)
	}
	hasModel := request.ExpectedStatement != nil
	hasJSON := len(request.ExpectedStatementJSON) > 0
	if !hasModel && !hasJSON {
		return Result{}, newError(idInputUnavailable, "expected_statement", "an expected Statement is required", nil)
	}
	if hasModel && hasJSON {
		return Result{}, newError(idPolicySchemaInvalid, "expected_statement", "exactly one expected Statement representation is allowed", nil)
	}
	if trustedMaterial == nil {
		return Result{}, newError(idUngovernedTrustRoot, "trust_root", "authenticated trusted material is required", nil)
	}
	if err := validateIdentityExpectation(request.Identity); err != nil {
		return Result{}, err
	}
	parsed, err := ParseBundle(request.Bundle)
	if err != nil {
		return Result{}, err
	}
	if err := rejectLegacyOverrides(); err != nil {
		return Result{}, err
	}
	return verifyParsedWithTrustedMaterial(request, parsed, trustedMaterial)
}

func verifyParsedWithTrustedMaterial(request Request, parsed ParsedBundle, trustedMaterial root.TrustedMaterial) (Result, error) {
	entries, err := parsed.sigstore.TlogEntries()
	if err != nil || len(entries) == 0 {
		return Result{}, newError(idMissingRekorEntry, "bundle.verificationMaterial.tlogEntries", "bundle-contained Rekor evidence is required", err)
	}

	certificateIdentity, err := verify.NewShortCertificateIdentity(request.Identity.Issuer, "", request.Identity.SignerURI, "")
	if err != nil {
		return Result{}, newError(idVerificationModeInvalid, "identity", "signer identity expectation is invalid", err)
	}
	verifier, err := verify.NewVerifier(
		trustedMaterial,
		verify.WithSignedCertificateTimestamps(1),
		verify.WithTransparencyLog(1),
		verify.WithObserverTimestamps(1),
	)
	if err != nil {
		return Result{}, newError(idUngovernedTrustRoot, "trust_root", "governed trusted material cannot construct a verifier", err)
	}
	verified, err := verifier.Verify(
		parsed.sigstore,
		verify.NewPolicy(verify.WithoutArtifactUnsafe(), verify.WithCertificateIdentity(certificateIdentity)),
	)
	if err != nil {
		return Result{}, classifyVerificationError(err)
	}

	verificationContent, err := parsed.sigstore.VerificationContent()
	if err != nil || verificationContent.Certificate() == nil {
		return Result{}, newError(idSignatureMismatch, "bundle.verificationMaterial.certificate", "verified Fulcio certificate is unavailable", err)
	}
	certificate := verificationContent.Certificate()
	if err := verifyIdentity(certificate, request.Identity); err != nil {
		return Result{}, err
	}
	signingTime, err := verifiedSigningTime(verified, certificate)
	if err != nil {
		return Result{}, err
	}
	if request.ExpectedStatement != nil {
		if err := CompareStatement(parsed.statement, *request.ExpectedStatement); err != nil {
			return Result{}, err
		}
	}
	if len(request.ExpectedStatementJSON) > 0 {
		equal, err := canonicaljson.Equal(parsed.statement, request.ExpectedStatementJSON)
		if err != nil {
			return Result{}, parseError("expected_statement", err)
		}
		if !equal {
			return Result{}, newError(idStatementAssemblyMismatch, "statement", "emitted Statement differs from expected JSON", nil)
		}
	}
	return Result{
		bundle:      parsed.BundleBytes(),
		statement:   parsed.StatementBytes(),
		Certificate: certificate,
		SigningTime: signingTime,
	}, nil
}

func acquireTrustedMaterial(request Request, verificationTime time.Time) (root.TrustedMaterial, error) {
	switch request.Mode {
	case ModeOnline:
		if err := validateOnlineRoot(request.TrustRoot); err != nil {
			return nil, err
		}
		trustedRoot, err := root.FetchTrustedRoot()
		if err != nil {
			return nil, newError(idUngovernedTrustRoot, "trust_root", "Sigstore public-good TUF authentication failed", err)
		}
		return trustedRoot, nil
	case ModeOffline:
		return acquirePinnedRoot(request, verificationTime)
	default:
		return nil, newError(idVerificationModeInvalid, "verification_mode", "exactly one online or offline mode is required", nil)
	}
}

func validateOnlineRoot(trustRoot policy.TrustRoot) error {
	if trustRoot.Mode != "tuf" || trustRoot.Instance != publicGoodInstance || trustRoot.Path != nil ||
		trustRoot.SHA256 != nil || trustRoot.TUFRepository != nil || trustRoot.RevalidatedAt != nil || trustRoot.RefreshBefore != nil {
		return newError(idVerificationModeInvalid, "trust_root", "online mode requires only the Sigstore public-good TUF root", nil)
	}
	return nil
}

func acquirePinnedRoot(request Request, verificationTime time.Time) (root.TrustedMaterial, error) {
	metadata := request.TrustRoot
	if metadata.Mode != "pinned" || metadata.Instance != publicGoodInstance || metadata.Path == nil || *metadata.Path == "" ||
		metadata.SHA256 == nil || metadata.TUFRepository == nil || metadata.RevalidatedAt == nil || metadata.RefreshBefore == nil {
		return nil, newError(idVerificationModeInvalid, "trust_root", "offline mode requires the complete pinned-root shape", nil)
	}
	if len(request.PinnedRoot) == 0 {
		return nil, newError(idInputUnavailable, "trust_root", "pinned trusted-root bytes are required", nil)
	}
	if *metadata.TUFRepository != publicGoodTUF || sha256Hex(request.PinnedRoot) != *metadata.SHA256 {
		return nil, newError(idUngovernedTrustRoot, "trust_root", "pinned root digest or TUF repository identity is not governed", nil)
	}
	if err := canonicaljson.Validate(request.PinnedRoot); err != nil {
		if diagnosticID(err) == idDuplicateJSONMember {
			return nil, parseError("trust_root", err)
		}
		return nil, newError(idUngovernedTrustRoot, "trust_root", "pinned trusted-root JSON is malformed", err)
	}
	revalidatedAt, err := time.Parse(policyTimestampLayout, *metadata.RevalidatedAt)
	if err != nil {
		return nil, newError(idVerificationModeInvalid, "trust_root.revalidated_at", "pinned-root timestamp is malformed", err)
	}
	refreshBefore, err := time.Parse(policyTimestampLayout, *metadata.RefreshBefore)
	if err != nil || !refreshBefore.After(revalidatedAt) || verificationTime.IsZero() {
		return nil, newError(idVerificationModeInvalid, "trust_root.refresh_before", "pinned-root freshness inputs are malformed", err)
	}
	if verificationTime.After(refreshBefore) {
		return nil, newError(idStalePinnedTrustRoot, "trust_root.refresh_before", "pinned trusted root is stale at verification time", nil)
	}
	trustedRoot, err := root.NewTrustedRootFromJSON(request.PinnedRoot)
	if err != nil {
		return nil, newError(idUngovernedTrustRoot, "trust_root", "pinned trusted root cannot be loaded", err)
	}
	return trustedRoot, nil
}

func rejectLegacyOverrides() error {
	for _, name := range forbiddenTrustRootOverrides {
		if _, present := os.LookupEnv(name); present {
			return newError(idLegacyTrustRootOverride, "environment."+name, "legacy Sigstore component override is forbidden", nil)
		}
	}
	return nil
}

func classifyVerificationError(err error) error {
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "signed certificate timestamp") || strings.Contains(message, "sct"):
		return newError(idMissingSCT, "bundle.verificationMaterial.certificate.sct", "Fulcio SCT is missing or invalid", err)
	case strings.Contains(message, "integrated time outside certificate validity") || strings.Contains(message, "signature time outside"):
		return newError(idSignatureTimeViolation, "bundle.verificationMaterial.tlogEntries.integratedTime", "signature time is outside certificate validity", err)
	case strings.Contains(message, "log inclusion") || strings.Contains(message, "transparency log"):
		return newError(idMissingRekorEntry, "bundle.verificationMaterial.tlogEntries", "Rekor inclusion or SET evidence is missing or invalid", err)
	case strings.Contains(message, "identity"):
		return newError(idSignerIdentityUntrusted, "bundle.verificationMaterial.certificate", "signer identity is not trusted", err)
	default:
		return newError(idSignatureMismatch, "bundle", "bundle cryptographic verification failed", err)
	}
}

func verifiedSigningTime(result *verify.VerificationResult, certificate *x509.Certificate) (time.Time, error) {
	for _, timestamp := range result.VerifiedTimestamps {
		if timestamp.Type != "Tlog" {
			continue
		}
		if err := validateSigningTime(certificate, timestamp.Timestamp); err != nil {
			return time.Time{}, err
		}
		return timestamp.Timestamp.UTC(), nil
	}
	return time.Time{}, newError(idSignatureTimeViolation, "bundle.verificationMaterial.tlogEntries.integratedTime", "verified SET-covered integrated time is required", nil)
}

func validateSigningTime(certificate *x509.Certificate, signingTime time.Time) error {
	if certificate == nil || signingTime.Before(certificate.NotBefore) || signingTime.After(certificate.NotAfter) {
		return newError(idSignatureTimeViolation, "bundle.verificationMaterial.tlogEntries.integratedTime", "SET-covered signing time is outside the Fulcio certificate validity interval", nil)
	}
	return nil
}

func verifyIdentity(certificate *x509.Certificate, expected IdentityExpectation) error {
	if certificate == nil {
		return newError(idSignerIdentityClaimMissing, "certificate", "verified signer certificate is missing", nil)
	}
	if len(certificate.URIs) != 1 || certificate.URIs[0].String() != expected.SignerURI {
		return newError(idSignerWorkflowPathMismatch, "certificate.san", "certificate URI SAN does not match the expected signer workflow", nil)
	}
	claims := []struct {
		oid      asn1.ObjectIdentifier
		expected string
		id       string
		field    string
	}{
		{oidIssuerV2, expected.Issuer, idIssuerMismatch, "certificate.issuer"},
		{oidBuildSignerURI, expected.SignerURI, idSignerWorkflowPathMismatch, "certificate.build_signer_uri"},
		{oidBuildSignerDigest, expected.WorkflowSHA, idSignerWorkflowSHAMismatch, "certificate.build_signer_digest"},
		{oidRunnerEnvironment, expected.RunnerEnvironment, idSelfHostedRunner, "certificate.runner_environment"},
		{oidSourceRepositoryURI, expected.SourceRepositoryURI, idSourceIdentityMismatch, "certificate.source_repository_uri"},
		{oidSourceDigest, expected.SourceDigest, idSourceDigestMismatch, "certificate.source_digest"},
		{oidSourceRef, expected.SourceRef, idSourceRefMismatch, "certificate.source_ref"},
		{oidSourceRepositoryID, expected.SourceRepositoryID, idSourceNumericIDMismatch, "certificate.source_repository_id"},
		{oidSourceOwnerID, expected.SourceRepositoryOwnerID, idSourceNumericIDMismatch, "certificate.source_repository_owner_id"},
		{oidRunInvocationURI, expected.RunInvocationURI, idRunInvocationURIInvalid, "certificate.run_invocation_uri"},
	}
	for _, claim := range claims {
		actual, present, err := certificateExtension(certificate, claim.oid)
		if err != nil || !present {
			return newError(idSignerIdentityClaimMissing, claim.field, "required signer identity claim is absent or malformed", err)
		}
		if actual != claim.expected {
			return newError(claim.id, claim.field, "verified signer identity claim does not match policy", nil)
		}
	}
	if err := identity.ValidateCanonicalRepositoryURI(expected.SourceRepositoryURI); err != nil {
		return newError(idVerificationModeInvalid, "identity.source_repository_uri", "expected source repository URI is not canonical", err)
	}
	if err := identity.ValidateFullSHA(expected.WorkflowSHA); err != nil {
		return newError(idVerificationModeInvalid, "identity.workflow_sha", "expected workflow SHA is not immutable", err)
	}
	if err := identity.ValidateFullSHA(expected.SourceDigest); err != nil {
		return newError(idVerificationModeInvalid, "identity.source_digest", "expected source digest is not immutable", err)
	}
	if _, err := identity.ParseRunInvocationURI(expected.RunInvocationURI, expected.SourceRepositoryURI); err != nil {
		return newError(idRunInvocationURIInvalid, "identity.run_invocation_uri", "expected Run Invocation URI is not canonical", err)
	}
	return nil
}

func validateIdentityExpectation(expected IdentityExpectation) error {
	if expected.Issuer != "https://token.actions.githubusercontent.com" {
		return newError(idPolicySchemaInvalid, "identity.issuer", "expected issuer must be GitHub Actions", nil)
	}
	if err := validateSignerURI(expected.SignerURI); err != nil {
		return newError(idPolicySchemaInvalid, "identity.signer_uri", "expected signer URI is not canonical", err)
	}
	if err := identity.ValidateCanonicalRepositoryURI(expected.SourceRepositoryURI); err != nil {
		return newError(idPolicySchemaInvalid, "identity.source_repository_uri", "expected source repository URI is not canonical", err)
	}
	if err := identity.ValidateFullSHA(expected.WorkflowSHA); err != nil {
		return newError(idPolicySchemaInvalid, "identity.workflow_sha", "expected workflow SHA is not immutable", err)
	}
	if err := identity.ValidateFullSHA(expected.SourceDigest); err != nil {
		return newError(idPolicySchemaInvalid, "identity.source_digest", "expected source digest is not immutable", err)
	}
	if !positiveDecimalPattern.MatchString(expected.SourceRepositoryID) ||
		!positiveDecimalPattern.MatchString(expected.SourceRepositoryOwnerID) {
		return newError(idPolicySchemaInvalid, "identity.source_numeric_ids", "expected source numeric IDs must be positive decimal strings", nil)
	}
	if !strings.HasPrefix(expected.SourceRef, "refs/") || strings.TrimSpace(expected.SourceRef) != expected.SourceRef {
		return newError(idPolicySchemaInvalid, "identity.source_ref", "expected source ref must be a full canonical ref", nil)
	}
	if expected.RunnerEnvironment != "github-hosted" {
		return newError(idPolicySchemaInvalid, "identity.runner_environment", "expected runner environment must be github-hosted", nil)
	}
	if _, err := identity.ParseRunInvocationURI(expected.RunInvocationURI, expected.SourceRepositoryURI); err != nil {
		return newError(idPolicySchemaInvalid, "identity.run_invocation_uri", "expected Run Invocation URI is not canonical", err)
	}
	return nil
}

func validateSignerURI(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host != "github.com" || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Opaque != "" {
		return fmt.Errorf("signer URI has forbidden components")
	}
	workflowPath, ref, found := strings.Cut(strings.TrimPrefix(parsed.Path, "/"), "@")
	components := strings.Split(workflowPath, "/")
	if len(components) != 5 || components[0] == "" || components[1] == "" ||
		components[2] != ".github" || components[3] != "workflows" {
		return fmt.Errorf("signer URI does not identify one GitHub workflow")
	}
	filename := components[4]
	if !found || strings.Contains(ref, "@") || (!strings.HasSuffix(filename, ".yml") && !strings.HasSuffix(filename, ".yaml")) ||
		!strings.HasPrefix(ref, "refs/") {
		return fmt.Errorf("signer URI workflow or ref is not canonical")
	}
	return nil
}

func certificateExtension(certificate *x509.Certificate, oid asn1.ObjectIdentifier) (string, bool, error) {
	var matched []byte
	for _, extension := range certificate.Extensions {
		if !extension.Id.Equal(oid) {
			continue
		}
		if matched != nil {
			return "", true, fmt.Errorf("certificate extension %s is duplicated", oid.String())
		}
		matched = extension.Value
	}
	if matched == nil {
		return "", false, nil
	}
	var value string
	rest, err := asn1.Unmarshal(matched, &value)
	if err != nil || len(rest) != 0 || value == "" {
		return "", true, fmt.Errorf("decode certificate extension %s: %w", oid.String(), err)
	}
	return value, true, nil
}
