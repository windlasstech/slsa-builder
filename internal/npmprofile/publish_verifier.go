package npmprofile

import (
	"context"
	"errors"
	"strings"

	"github.com/windlasstech/slsa-builder/internal/attestation"
	"github.com/windlasstech/slsa-builder/internal/identity"
	"github.com/windlasstech/slsa-builder/internal/policy"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

// SigstorePublishVerifierConfig fixes the trust root and all identity claims except run attempt.
type SigstorePublishVerifierConfig struct {
	Mode       attestation.Mode
	TrustRoot  policy.TrustRoot
	PinnedRoot []byte
	Identity   attestation.IdentityExpectation
}

type sigstorePublishVerifier struct {
	config SigstorePublishVerifierConfig
}

// NewSigstorePublishBundleVerifier creates the production verifier used for local and published bundles.
func NewSigstorePublishBundleVerifier(config SigstorePublishVerifierConfig) PublishBundleVerifier {
	config.PinnedRoot = append([]byte(nil), config.PinnedRoot...)
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
	if err := verifier.validateStatement(statement); err != nil {
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
	if parameters.Source.Repository != expected.SourceRepositoryURI {
		return attestation.IdentityExpectation{}, npmValidationError("windlass.verify.error.source-identity-mismatch", "externalParameters.source.repository", "signed npm source repository differs from the verified repository identity")
	}
	invocationRef := parameters.Source.Ref
	invocationRevision := parameters.Source.Revision
	if parameters.Source.InvocationRef != nil {
		invocationRef = *parameters.Source.InvocationRef
		invocationRevision = *parameters.Source.InvocationRevision
	}
	if expected.SourceRef != invocationRef {
		return attestation.IdentityExpectation{}, npmValidationError("windlass.verify.error.source-ref-mismatch", "externalParameters.source.invocation_ref", "signed npm invocation ref differs from the verified invocation identity")
	}
	if expected.SourceDigest != invocationRevision {
		return attestation.IdentityExpectation{}, npmValidationError("windlass.verify.error.source-digest-mismatch", "externalParameters.source.invocation_revision", "signed npm invocation revision differs from the verified invocation identity")
	}
	expected.SourceRef = invocationRef
	expected.SourceDigest = invocationRevision
	return expected, nil
}

func (verifier *sigstorePublishVerifier) validateStatement(statement provenance.Statement) error {
	parameters, err := DecodeExternalParameters(statement.Predicate.BuildDefinition.ExternalParameters)
	if err != nil {
		return err
	}
	identityExpectation := verifier.config.Identity
	if parameters.Source.Repository != identityExpectation.SourceRepositoryURI {
		return errors.New("signed npm source repository differs from verified Fulcio identity")
	}
	builderID := statement.Predicate.RunDetails.Builder.ID
	builderPath, builderSHA, builderFound := strings.Cut(builderID, "@")
	signerPath, _, signerFound := strings.Cut(identityExpectation.SignerURI, "@")
	if !builderFound || !signerFound || builderPath != signerPath || builderSHA != identityExpectation.WorkflowSHA {
		return errors.New("signed npm builder identity differs from verified Fulcio signer")
	}
	if len(statement.Subject) != 1 {
		return errors.New("signed npm provenance must contain exactly one subject")
	}
	corepackVersion := (*string)(nil)
	if parameters.PackageManager.Name != ManagerNPM {
		value, present := statement.Predicate.RunDetails.Builder.Version["corepack"]
		if !present {
			return errors.New("signed npm builder is missing required Corepack version")
		}
		corepackVersion = &value
	}
	input := NPMProvenanceInput{
		BuildMetadata: BuildMetadata{
			SchemaVersion: "1",
			PrimaryArtifact: PrimaryArtifact{
				ArtifactName:    "registry-published-attestation",
				PayloadFileName: parameters.Package.TarballName,
				SHA256:          statement.Subject[0].Digest["sha256"],
				SHA512:          statement.Subject[0].Digest["sha512"],
			},
			ExternalParameters:   statement.Predicate.BuildDefinition.ExternalParameters,
			ResolvedDependencies: statement.Predicate.BuildDefinition.ResolvedDependencies,
		},
		BuilderID:             builderID,
		NodeJSVersion:         "v" + parameters.Runtime.NodeVersion,
		CorepackVersion:       corepackVersion,
		InvocationID:          statement.Predicate.RunDetails.Metadata.InvocationID,
		StartedOn:             statement.Predicate.RunDetails.Metadata.StartedOn,
		FinishedOn:            statement.Predicate.RunDetails.Metadata.FinishedOn,
		RuntimeReleaseRef:     parameters.Source.Ref,
		PeeledReleaseRevision: parameters.Source.Revision,
	}
	return ValidateNPMStatement(statement, input)
}
