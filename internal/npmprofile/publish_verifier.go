package npmprofile

import (
	"context"
	"errors"
	"strings"

	"github.com/windlasstech/slsa-builder/internal/attestation"
	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
	"github.com/windlasstech/slsa-builder/internal/identity"
	"github.com/windlasstech/slsa-builder/internal/policy"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

// SigstorePublishVerifierConfig fixes the trust root and all identity claims except run attempt.
type SigstorePublishVerifierConfig struct {
	Mode          attestation.Mode
	TrustRoot     policy.TrustRoot
	PinnedRoot    []byte
	Identity      attestation.IdentityExpectation
	BuildMetadata BuildMetadata
	Packed        PackedMetadata
}

type sigstorePublishVerifier struct {
	config SigstorePublishVerifierConfig
}

// ValidateConsumerStatement applies offline npm semantics after independent identity authorization.
func ValidateConsumerStatement(statement provenance.Statement, config SigstorePublishVerifierConfig) error {
	verifier := sigstorePublishVerifier{config: config}
	if _, err := verifier.identityForStatement(statement); err != nil {
		return err
	}
	return verifier.validateStatement(statement)
}

// NewSigstorePublishBundleVerifier creates the production verifier used for local and published bundles.
func NewSigstorePublishBundleVerifier(config SigstorePublishVerifierConfig) PublishBundleVerifier {
	config.PinnedRoot = append([]byte(nil), config.PinnedRoot...)
	config.BuildMetadata.ExternalParameters = append([]byte(nil), config.BuildMetadata.ExternalParameters...)
	config.BuildMetadata.ResolvedDependencies = cloneNPMDependencies(config.BuildMetadata.ResolvedDependencies)
	config.Packed.Files = append([]string(nil), config.Packed.Files...)
	config.Packed.ConsumerSurface = cloneRawMap(config.Packed.ConsumerSurface)
	return &sigstorePublishVerifier{config: config}
}

func (verifier *sigstorePublishVerifier) Verify(ctx context.Context, bundleBytes []byte) (VerifiedPublishBundle, error) {
	if verifier == nil {
		return VerifiedPublishBundle{}, errors.New("sigstore publish verifier is required")
	}
	parsed, err := attestation.ParseBundle(bundleBytes)
	if err != nil {
		return VerifiedPublishBundle{}, err
	}
	statement, err := provenance.DecodeStatement(parsed.StatementBytes())
	if err != nil {
		return VerifiedPublishBundle{}, err
	}
	expectedIdentity, err := verifier.identityForStatement(statement)
	if err != nil {
		return VerifiedPublishBundle{}, err
	}
	runInvocation := statement.Predicate.RunDetails.Metadata.InvocationID
	if _, err := identity.ParseRunInvocationURI(runInvocation, verifier.config.Identity.SourceRepositoryURI); err != nil {
		return VerifiedPublishBundle{}, err
	}
	expectedIdentity.RunInvocationURI = runInvocation
	if err := verifier.validateStatement(statement); err != nil {
		return VerifiedPublishBundle{}, err
	}
	if _, err := attestation.Verify(ctx, attestation.Request{
		Mode:                  verifier.config.Mode,
		Bundle:                bundleBytes,
		TrustRoot:             verifier.config.TrustRoot,
		PinnedRoot:            verifier.config.PinnedRoot,
		Identity:              expectedIdentity,
		ExpectedStatementJSON: parsed.StatementBytes(),
	}); err != nil {
		return VerifiedPublishBundle{}, err
	}
	return VerifiedPublishBundle{Statement: statement, RunInvocationURI: runInvocation}, nil
}

func (verifier *sigstorePublishVerifier) identityForStatement(statement provenance.Statement) (attestation.IdentityExpectation, error) {
	parameters, err := DecodeExternalParameters(statement.Predicate.BuildDefinition.ExternalParameters)
	if err != nil {
		return attestation.IdentityExpectation{}, err
	}
	expected := verifier.config.Identity
	sources, err := ResolveStatementSources(statement)
	if err != nil {
		return attestation.IdentityExpectation{}, err
	}
	if sources.Repository != expected.SourceRepositoryURI {
		return attestation.IdentityExpectation{}, npmValidationError("windlass.verify.error.source-identity-mismatch", "externalParameters.workflow.repository", "signed npm repository differs from the verified repository identity")
	}
	invocationRevision := sources.InvocationCommit
	if expected.SourceRef != parameters.Workflow.Ref {
		return attestation.IdentityExpectation{}, npmValidationError("windlass.verify.error.source-ref-mismatch", "externalParameters.workflow.ref", "signed npm invocation ref differs from the verified invocation identity")
	}
	if expected.SourceDigest != invocationRevision {
		return attestation.IdentityExpectation{}, npmValidationError("windlass.verify.error.source-digest-mismatch", "resolvedDependencies.invocation-source.digest.gitCommit", "signed npm invocation revision differs from the verified invocation identity")
	}
	internal, err := provenance.DecodeInternalParameters(statement.Predicate.BuildDefinition.InternalParameters)
	if err != nil {
		return attestation.IdentityExpectation{}, err
	}
	if expected.BuildTrigger == "" || internal.GitHub.EventName != expected.BuildTrigger {
		return attestation.IdentityExpectation{}, npmValidationError("windlass.verify.error.build-trigger-mismatch", "internalParameters.github.event_name", "signed event differs from required invocation expectation")
	}
	expected.SourceRef = parameters.Workflow.Ref
	expected.SourceDigest = invocationRevision
	expected.BuildTrigger = internal.GitHub.EventName
	expected.BuildConfigURI = expected.SourceRepositoryURI + "/" + parameters.Workflow.Path + "@" + parameters.Workflow.Ref
	return expected, nil
}

func (verifier *sigstorePublishVerifier) validateStatement(statement provenance.Statement) error {
	parameters, err := DecodeExternalParameters(statement.Predicate.BuildDefinition.ExternalParameters)
	if err != nil {
		return err
	}
	identityExpectation := verifier.config.Identity
	if parameters.Workflow.Repository != identityExpectation.SourceRepositoryURI {
		return npmValidationError("windlass.verify.error.source-identity-mismatch", "externalParameters.workflow.repository", "signed npm source repository differs from verified Fulcio identity")
	}
	builderID := statement.Predicate.RunDetails.Builder.ID
	if err := identity.ValidateBuilderIDForWorkflow(builderID, NPMWorkflowPath); err != nil {
		return err
	}
	builderPath, builderSHA, builderFound := strings.Cut(builderID, "@")
	signerPath, _, signerFound := strings.Cut(identityExpectation.SignerURI, "@")
	if !builderFound || !signerFound {
		return npmValidationError("windlass.verify.error.builder-id-not-immutable", "predicate.runDetails.builder.id", "signed npm builder identity is malformed")
	}
	if builderPath != signerPath {
		return npmValidationError("windlass.verify.error.signer-workflow-path-mismatch", "predicate.runDetails.builder.id", "signed npm builder path differs from verified Fulcio signer")
	}
	if builderSHA != identityExpectation.WorkflowSHA {
		return npmValidationError("windlass.verify.error.signer-workflow-sha-mismatch", "predicate.runDetails.builder.id", "signed npm builder SHA differs from verified Fulcio signer")
	}
	if len(statement.Subject) != 1 {
		return npmValidationError("windlass.verify.error.subject-cardinality-error", "subject", "signed npm provenance must contain exactly one subject")
	}
	corepackVersion := (*string)(nil)
	dependencies, err := finalDependencySet(statement.Predicate.BuildDefinition.ResolvedDependencies)
	if err != nil {
		return err
	}
	if _, present := dependencies["package-manager-distribution"]; present {
		value, present := statement.Predicate.RunDetails.Builder.Version["corepack"]
		if !present {
			return npmValidationError("windlass.verify.error.builder-version-mismatch", "runDetails.builder.version", "signed npm builder is missing required Corepack version")
		}
		corepackVersion = &value
	}
	metadata := verifier.config.BuildMetadata
	if _, err := metadata.ManifestEvidence.Decode(parameters.Package.Directory); err != nil {
		return err
	}
	if equal, err := canonicaljson.Equal(metadata.ExternalParameters, statement.Predicate.BuildDefinition.ExternalParameters); err != nil || !equal {
		return npmValidationError(IDUnexpectedExternalParameters, "externalParameters", "signed inputs differ from the verified build metadata")
	}
	expectedStatement := provenance.Statement{Predicate: provenance.Predicate{BuildDefinition: provenance.BuildDefinition{ExternalParameters: metadata.ExternalParameters, ResolvedDependencies: metadata.ResolvedDependencies}}}
	expectedSources, err := ResolveStatementSources(expectedStatement)
	if err != nil {
		return err
	}
	input := NPMProvenanceInput{
		BuildMetadata: BuildMetadata{
			SchemaVersion:          "1",
			PrimaryArtifact:        metadata.PrimaryArtifact,
			ManifestEvidence:       metadata.ManifestEvidence,
			BuildScriptDisposition: metadata.BuildScriptDisposition,
			ExternalParameters:     metadata.ExternalParameters,
			ResolvedDependencies:   metadata.ResolvedDependencies,
		},
		BuilderID: builderID,
		GitHub: provenance.GitHubParameters{
			EventName:         identityExpectation.BuildTrigger,
			RepositoryID:      identityExpectation.SourceRepositoryID,
			RepositoryOwnerID: identityExpectation.SourceRepositoryOwnerID,
		},
		NodeJSVersion:         statement.Predicate.RunDetails.Builder.Version["nodejs"],
		NPMVersion:            statement.Predicate.RunDetails.Builder.Version["npm"],
		Packed:                verifier.config.Packed,
		InvocationRevision:    identityExpectation.SourceDigest,
		CorepackVersion:       corepackVersion,
		InvocationID:          statement.Predicate.RunDetails.Metadata.InvocationID,
		StartedOn:             statement.Predicate.RunDetails.Metadata.StartedOn,
		FinishedOn:            statement.Predicate.RunDetails.Metadata.FinishedOn,
		RuntimeReleaseRef:     expectedSources.BuiltRef,
		PeeledReleaseRevision: expectedSources.BuiltCommit,
	}
	return ValidateNPMStatement(statement, input)
}
