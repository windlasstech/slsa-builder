// Package attestation verifies preserved Sigstore bundles and their signed in-toto Statements.
package attestation

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
	"github.com/windlasstech/slsa-builder/internal/diagnostic"
	"github.com/windlasstech/slsa-builder/internal/policy"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

const (
	idDuplicateJSONMember        = "windlass.verify.error.duplicate-json-member"
	idInputUnavailable           = "windlass.verify.error.input-unavailable"
	idIssuerMismatch             = "windlass.verify.error.issuer-mismatch"
	idLegacyTrustRootOverride    = "windlass.verify.error.legacy-trust-root-override"
	idMissingRekorEntry          = "windlass.verify.error.missing-rekor-entry"
	idMissingSCT                 = "windlass.verify.error.missing-sct"
	idPolicySchemaInvalid        = "windlass.verify.error.policy-schema-invalid"
	idRunInvocationURIInvalid    = "windlass.verify.error.run-invocation-uri-invalid"
	idSelfHostedRunner           = "windlass.verify.error.self-hosted-runner"
	idSignatureInvalid           = "windlass.verify.error.signature-invalid"
	idSignatureMismatch          = "windlass.verify.error.signature-mismatch"
	idSignatureTimeViolation     = "windlass.verify.error.signature-time-violation"
	idSignerIdentityClaimMissing = "windlass.verify.error.signer-identity-claim-missing"
	idSignerIdentityUntrusted    = "windlass.verify.error.signer-identity-untrusted"
	idSignerWorkflowPathMismatch = "windlass.verify.error.signer-workflow-path-mismatch"
	idSignerWorkflowSHAMismatch  = "windlass.verify.error.signer-workflow-sha-mismatch"
	idSourceDigestMismatch       = "windlass.verify.error.source-digest-mismatch"
	idSourceIdentityMismatch     = "windlass.verify.error.source-identity-mismatch"
	idSourceNumericIDMismatch    = "windlass.verify.error.source-numeric-id-mismatch"
	idSourceRefMismatch          = "windlass.verify.error.source-ref-mismatch"
	idStalePinnedTrustRoot       = "windlass.verify.error.stale-pinned-trust-root"
	idStatementAssemblyMismatch  = "windlass.verify.error.statement-assembly-mismatch"
	idUngovernedTrustRoot        = "windlass.verify.error.ungoverned-trust-root"
	idVerificationModeInvalid    = "windlass.verify.error.verification-mode-invalid"
	idVerifierExecutionFailure   = "windlass.verify.error.verifier-execution-failure"
)

// Mode selects the governed trust-root acquisition path for one verification invocation.
type Mode string

const (
	// ModeOnline authenticates the current Sigstore public-good root through TUF.
	ModeOnline Mode = "online"
	// ModeOffline uses only caller-supplied pinned trusted-root bytes.
	ModeOffline Mode = "offline"
)

// IdentityExpectation contains the semantic GitHub Actions certificate values required by ADR 0068.
type IdentityExpectation struct {
	Issuer                  string
	SignerURI               string
	WorkflowSHA             string
	SourceRepositoryURI     string
	SourceRepositoryID      string
	SourceRepositoryOwnerID string
	SourceDigest            string
	SourceRef               string
	RunnerEnvironment       string
	RunInvocationURI        string
}

// Request is the complete immutable input set for one bundle verification decision.
type Request struct {
	Mode                  Mode
	Bundle                []byte
	TrustRoot             policy.TrustRoot
	PinnedRoot            []byte
	Identity              IdentityExpectation
	ExpectedStatement     *provenance.Statement
	ExpectedStatementJSON []byte
}

// Result contains authenticated evidence while retaining the exact input bundle and payload bytes.
type Result struct {
	bundle      []byte
	statement   []byte
	Certificate *x509.Certificate
	SigningTime time.Time
}

// BundleBytes returns a defensive copy of the exact actions/attest bytes supplied to verification.
func (result Result) BundleBytes() []byte {
	return append([]byte(nil), result.bundle...)
}

// StatementBytes returns a defensive copy of the exact signed DSSE payload bytes.
func (result Result) StatementBytes() []byte {
	return append([]byte(nil), result.statement...)
}

// ParsedBundle retains strict-parsed Sigstore material alongside exact serialized input bytes.
type ParsedBundle struct {
	raw       []byte
	statement []byte
	sigstore  *bundle.Bundle
}

// BundleBytes returns a defensive copy of the preserved serialized bundle.
func (parsed ParsedBundle) BundleBytes() []byte {
	return append([]byte(nil), parsed.raw...)
}

// StatementBytes returns a defensive copy of the decoded DSSE payload.
func (parsed ParsedBundle) StatementBytes() []byte {
	return append([]byte(nil), parsed.statement...)
}

// ParseBundle rejects ambiguous JSON before sigstore-go parses the same exact bytes.
func ParseBundle(raw []byte) (ParsedBundle, error) {
	if len(raw) == 0 {
		return ParsedBundle{}, newError(idInputUnavailable, "bundle", "bundle bytes are required", nil)
	}
	if err := canonicaljson.Validate(raw); err != nil {
		return ParsedBundle{}, parseError("bundle", err)
	}

	parsed := &bundle.Bundle{}
	if err := parsed.UnmarshalJSON(raw); err != nil {
		return ParsedBundle{}, newError(idSignatureInvalid, "bundle", "Sigstore bundle is malformed", err)
	}
	envelope, err := parsed.Envelope()
	if err != nil {
		return ParsedBundle{}, newError(idSignatureInvalid, "bundle.dsseEnvelope", "bundle does not contain a valid DSSE envelope", err)
	}
	statement, err := envelope.RawEnvelope().DecodeB64Payload()
	if err != nil {
		return ParsedBundle{}, newError(idSignatureInvalid, "bundle.dsseEnvelope.payload", "DSSE payload is not valid base64", err)
	}
	if err := canonicaljson.Validate(statement); err != nil {
		return ParsedBundle{}, parseError("bundle.dsseEnvelope.payload", err)
	}
	if envelope.RawEnvelope().PayloadType != bundle.IntotoMediaType {
		return ParsedBundle{}, newError(idSignatureInvalid, "bundle.dsseEnvelope.payloadType", "DSSE payload type must be application/vnd.in-toto+json", nil)
	}
	return ParsedBundle{
		raw:       append([]byte(nil), raw...),
		statement: append([]byte(nil), statement...),
		sigstore:  parsed,
	}, nil
}

// CompareStatement proves structural equality between signed payload bytes and a provenance model.
func CompareStatement(actual []byte, expected provenance.Statement) error {
	if _, err := provenance.DecodeStatement(actual); err != nil {
		if diagnosticID(err) == idDuplicateJSONMember {
			return parseError("bundle.dsseEnvelope.payload", err)
		}
		return newError(idStatementAssemblyMismatch, "statement", "signed Statement does not satisfy the closed provenance model", err)
	}
	expectedJSON, err := expected.CanonicalJSON()
	if err != nil {
		return newError(idVerifierExecutionFailure, "expected_statement", "expected Statement cannot be serialized", err)
	}
	equal, err := canonicaljson.Equal(actual, expectedJSON)
	if err != nil {
		return parseError("statement", err)
	}
	if !equal {
		return newError(idStatementAssemblyMismatch, "statement", "emitted Statement differs from validated signing inputs", nil)
	}
	return nil
}

// VerificationError binds a failure to the shared stable diagnostic registry.
type VerificationError struct {
	Diagnostic diagnostic.Diagnostic
	Cause      error
}

func (err *VerificationError) Error() string {
	if err.Cause == nil {
		return err.Diagnostic.ID + ": " + err.Diagnostic.Message
	}
	return err.Diagnostic.ID + ": " + err.Diagnostic.Message + ": " + err.Cause.Error()
}

func (err *VerificationError) Unwrap() error { return err.Cause }

// DiagnosticID returns the registry ID used by reports and fixture assertions.
func (err *VerificationError) DiagnosticID() string { return err.Diagnostic.ID }

func newError(id, check, message string, cause error) error {
	entry, err := diagnostic.New(id, check, message)
	if err != nil {
		return fmt.Errorf("construct attestation diagnostic %q: %w", id, err)
	}
	entry.Field = check
	return &VerificationError{Diagnostic: entry, Cause: cause}
}

func parseError(check string, err error) error {
	if diagnosticID(err) == idDuplicateJSONMember {
		return newError(idDuplicateJSONMember, check, "JSON contains a duplicate object member", err)
	}
	return newError(idSignatureInvalid, check, "security-relevant JSON is malformed", err)
}

func diagnosticID(err error) string {
	type diagnosticError interface{ DiagnosticID() string }
	var identified diagnosticError
	if errors.As(err, &identified) {
		return identified.DiagnosticID()
	}
	return ""
}

func sha256Hex(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
