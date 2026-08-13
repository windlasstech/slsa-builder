package command

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/windlasstech/slsa-builder/internal/attestation"
	"github.com/windlasstech/slsa-builder/internal/diagnostic"
	"github.com/windlasstech/slsa-builder/internal/digest"
	"github.com/windlasstech/slsa-builder/internal/policy"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

type verifyAttestationCommand struct{}

// NewVerifyAttestationCommand creates the internal Sigstore bundle verification subcommand.
func NewVerifyAttestationCommand() Command { return verifyAttestationCommand{} }

func (verifyAttestationCommand) Name() string { return "verify-attestation" }

func (verifyAttestationCommand) Execute(ctx context.Context, args []string, out io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	flags := flag.NewFlagSet("verify-attestation", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	bundlePath := flags.String("bundle", "", "preserved actions/attest bundle path")
	policyPath := flags.String("policy", "", "typed verifier policy JSON path")
	identityPath := flags.String("identity", "", "typed signer identity expectation JSON path")
	statementPath := flags.String("statement", "", "expected in-toto Statement JSON path")
	pinnedRootPath := flags.String("pinned-root", "", "offline pinned trusted-root JSON path")
	githubOutput := flags.String("github-output", os.Getenv("GITHUB_OUTPUT"), "GitHub Actions output file")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, writeErr := fmt.Fprintln(out, "Internal workflow command. Usage: slsa-builder-internal verify-attestation --bundle <json> --policy <json> --identity <json> --statement <json> [--pinned-root <json>] [--github-output <path>]")
			return writeErr
		}
		return err
	}
	if flags.NArg() != 0 || *bundlePath == "" || *policyPath == "" || *identityPath == "" || *statementPath == "" {
		return errors.New("--bundle, --policy, --identity, and --statement are required with no positional arguments")
	}

	bundleBytes, err := readRegularFile(*bundlePath)
	if err != nil {
		return emitFailure(out, diagnostic.IDInputUnavailable, "bundle", "attestation bundle is unavailable", ErrInvocationFailure)
	}
	policyBytes, err := readRegularFile(*policyPath)
	if err != nil {
		return emitFailure(out, diagnostic.IDInputUnavailable, "policy", "verifier policy is unavailable", ErrInvocationFailure)
	}
	explicitPolicy, err := policy.DecodeExplicitPolicy(policyBytes)
	if err != nil {
		return emitPolicyFailure(out, err)
	}
	identityBytes, err := readRegularFile(*identityPath)
	if err != nil {
		return emitFailure(out, diagnostic.IDInputUnavailable, "identity", "identity expectation is unavailable", ErrInvocationFailure)
	}
	identityExpectation, err := decodeTypedJSON[attestation.IdentityExpectation](identityBytes, nil)
	if err != nil {
		if id := diagnosticIDOf(err); id != "" {
			return emitFailure(out, id, "identity", "identity expectation contains ambiguous JSON", ErrVerificationFailure)
		}
		return emitFailure(out, "windlass.verify.error.policy-schema-invalid", "identity", "identity expectation violates the closed schema", ErrVerificationFailure)
	}
	expectedStatement, err := readRegularFile(*statementPath)
	if err != nil {
		return emitFailure(out, diagnostic.IDInputUnavailable, "statement", "expected Statement is unavailable", ErrInvocationFailure)
	}
	statement, err := provenance.DecodeStatement(expectedStatement)
	if err != nil {
		return emitFailure(out, "windlass.verify.error.statement-assembly-mismatch", "statement", "expected Statement does not satisfy the closed schema", ErrVerificationFailure)
	}
	if err := bindIdentityPolicy(identityExpectation, explicitPolicy, statement); err != nil {
		id := diagnosticIDOf(err)
		if id == "" {
			id = "windlass.verify.error.policy-schema-invalid"
		}
		return emitFailure(out, id, "identity", err.Error(), ErrVerificationFailure)
	}

	request := attestation.Request{
		Mode:                  attestation.ModeOnline,
		Bundle:                bundleBytes,
		TrustRoot:             explicitPolicy.TrustRoot,
		Identity:              identityExpectation,
		ExpectedStatementJSON: expectedStatement,
	}
	if explicitPolicy.TrustRoot.Mode == "pinned" {
		if *pinnedRootPath == "" {
			return emitFailure(out, diagnostic.IDInputUnavailable, "pinned-root", "offline policy requires --pinned-root", ErrInvocationFailure)
		}
		request.Mode = attestation.ModeOffline
		request.PinnedRoot, err = readRegularFile(*pinnedRootPath)
		if err != nil {
			return emitFailure(out, diagnostic.IDInputUnavailable, "pinned-root", "pinned trusted root is unavailable", ErrInvocationFailure)
		}
	} else if *pinnedRootPath != "" {
		return emitFailure(out, "windlass.verify.error.verification-mode-invalid", "pinned-root", "online policy forbids --pinned-root", ErrInvocationFailure)
	}

	result, err := attestation.Verify(ctx, request)
	if err != nil {
		return emitTypedVerificationFailure(out, err)
	}
	if *githubOutput != "" {
		if err := WriteGitHubOutputs(*githubOutput, attestationOutputAllowlist, map[string]string{
			"bundle-sha256":    digest.SumSHA256(result.BundleBytes()).String(),
			"result":           "pass",
			"run-invocation":   identityExpectation.RunInvocationURI,
			"statement-sha256": digest.SumSHA256(result.StatementBytes()).String(),
		}); err != nil {
			return emitFailure(out, diagnostic.IDVerifierExecutionFailure, "github-output", RedactSecrets(err.Error()), ErrInvocationFailure)
		}
	}
	return writeDiagnostics(out, &identityExpectation.RunInvocationURI, nil)
}

var attestationOutputAllowlist = NewOutputAllowlist("bundle-sha256", "result", "run-invocation", "statement-sha256")

// bindIdentityPolicy enforces the ADR 0080 binding model: the certificate identity expectation
// binds the signed invocation record (invocation_ref/invocation_revision when present, otherwise
// the built source fields), while the verifier policy source expectations bind the signed built
// source fields.
func bindIdentityPolicy(identity attestation.IdentityExpectation, explicit policy.ExplicitPolicy, statement provenance.Statement) error {
	source, err := decodeStatementSource(statement)
	if err != nil {
		return err
	}
	invocationRef, invocationRevision := source.Ref, source.Revision
	if source.InvocationRef != nil {
		invocationRef = *source.InvocationRef
	}
	if source.InvocationRevision != nil {
		invocationRevision = *source.InvocationRevision
	}
	if identity.SourceDigest != invocationRevision {
		return &identityBindingError{id: "windlass.verify.error.source-digest-mismatch", message: "identity expectation source digest differs from the signed invocation record"}
	}
	if identity.SourceRef != invocationRef {
		return &identityBindingError{id: "windlass.verify.error.source-ref-mismatch", message: "identity expectation source ref differs from the signed invocation record"}
	}
	if explicit.Source.RepositoryURI != source.Repository {
		return &identityBindingError{id: "windlass.verify.error.source-identity-mismatch", message: "verifier policy source repository differs from the signed built source"}
	}
	if explicit.Source.Digest != source.Revision {
		return &identityBindingError{id: "windlass.verify.error.source-digest-mismatch", message: "verifier policy source digest differs from the signed built source revision"}
	}
	if explicit.Source.Ref != source.Ref {
		return &identityBindingError{id: "windlass.verify.error.source-ref-mismatch", message: "verifier policy source ref differs from the signed built source ref"}
	}
	if identity.SourceRepositoryURI != explicit.Source.RepositoryURI ||
		identity.SourceRepositoryID != explicit.Source.RepositoryID ||
		identity.SourceRepositoryOwnerID != explicit.Source.RepositoryOwnerID ||
		identity.WorkflowSHA != explicit.Producer.WorkflowSHA ||
		identity.RunnerEnvironment != explicit.Producer.RunnerEnvironment {
		return errors.New("identity expectation conflicts with verifier policy")
	}
	wantWorkflow := "/" + strings.TrimPrefix(explicit.Producer.WorkflowPath, "/") + "@"
	if !strings.Contains(identity.SignerURI, wantWorkflow) {
		return errors.New("signer URI conflicts with verifier policy workflow path")
	}
	return nil
}

type identityBindingError struct {
	id      string
	message string
}

func (bindingError *identityBindingError) Error() string { return bindingError.message }

func (bindingError *identityBindingError) DiagnosticID() string { return bindingError.id }

type statementSourceFields struct {
	Repository         string  `json:"repository"`
	Ref                string  `json:"ref"`
	Revision           string  `json:"revision"`
	InvocationRef      *string `json:"invocation_ref"`
	InvocationRevision *string `json:"invocation_revision"`
}

func decodeStatementSource(statement provenance.Statement) (statementSourceFields, error) {
	var parameters struct {
		Source statementSourceFields `json:"source"`
	}
	if err := json.Unmarshal(statement.Predicate.BuildDefinition.ExternalParameters, &parameters); err != nil {
		return statementSourceFields{}, fmt.Errorf("decode statement source identity: %w", err)
	}
	if parameters.Source.Repository == "" || parameters.Source.Ref == "" || parameters.Source.Revision == "" {
		return statementSourceFields{}, errors.New("statement source identity is incomplete")
	}
	return parameters.Source, nil
}

func emitTypedVerificationFailure(out io.Writer, err error) error {
	var attestationError *attestation.VerificationError
	if errors.As(err, &attestationError) {
		if writeErr := writeDiagnostics(out, nil, []diagnostic.Diagnostic{attestationError.Diagnostic}); writeErr != nil {
			return writeErr
		}
		if definition, ok := diagnostic.Lookup(attestationError.Diagnostic.ID); ok && definition.ExitCode == diagnostic.ExitCodeInvocationFailure {
			return ErrInvocationFailure
		}
		return ErrVerificationFailure
	}
	var policyError *policy.ValidationError
	if errors.As(err, &policyError) {
		if writeErr := writeDiagnostics(out, nil, []diagnostic.Diagnostic{policyError.Diagnostic}); writeErr != nil {
			return writeErr
		}
		return ErrVerificationFailure
	}
	return emitFailure(out, diagnostic.IDVerifierExecutionFailure, "verify-attestation", "attestation verification could not be completed", ErrInvocationFailure)
}

func emitPolicyFailure(out io.Writer, err error) error {
	var policyError *policy.ValidationError
	if errors.As(err, &policyError) {
		return emitTypedVerificationFailure(out, err)
	}
	if id := diagnosticIDOf(err); id != "" {
		return emitFailure(out, id, "policy", "verifier policy contains ambiguous JSON", ErrVerificationFailure)
	}
	return emitFailure(out, "windlass.verify.error.policy-schema-invalid", "policy", "verifier policy violates the closed schema", ErrVerificationFailure)
}

func diagnosticIDOf(err error) string {
	type identified interface{ DiagnosticID() string }
	var diagnosticError identified
	if errors.As(err, &diagnosticError) {
		return diagnosticError.DiagnosticID()
	}
	return ""
}
