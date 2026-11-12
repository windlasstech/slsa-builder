package npmprofile

import (
	"github.com/windlasstech/slsa-builder/internal/digest"
	"github.com/windlasstech/slsa-builder/internal/identity"
	"github.com/windlasstech/slsa-builder/internal/toolchainpins"
)

type ProducerPackage struct {
	Name         string
	Version      string
	Publish      PublishIntent
	PackageURL   string
	UsesCorepack bool
}

func VerifyProducerPackage(metadata BuildMetadata) (ProducerPackage, error) {
	parameters, err := DecodeExternalParameters(metadata.ExternalParameters)
	if err != nil {
		return ProducerPackage{}, err
	}
	manifest, err := verifiedProducerManifest(metadata, parameters)
	if err != nil {
		return ProducerPackage{}, err
	}
	config, err := decodePublishConfig(manifest.PublishConfig)
	if err != nil {
		return ProducerPackage{}, err
	}
	publish, err := ResolvePublishIntent(stringOrEmpty(parameters.Publish.InputRegistryURL), stringOrEmpty(parameters.Publish.InputDistTag), stringOrEmpty(parameters.Publish.InputAccess), config)
	if err != nil {
		return ProducerPackage{}, err
	}
	url, err := npmRegistryPackageURL(publish.ResolvedRegistryURL, manifest.Name, manifest.Version)
	if err != nil {
		return ProducerPackage{}, err
	}
	result := ProducerPackage{Name: manifest.Name, Version: manifest.Version, Publish: publish, PackageURL: url}
	for _, descriptor := range metadata.ResolvedDependencies {
		if descriptor.Name == "package-manager-distribution" {
			result.UsesCorepack = true
		}
	}
	return result, nil
}

func VerifyPackedArtifact(filename string, expected PrimaryArtifact) (PackedMetadata, error) {
	packed, sha256, sha512, err := inspectTarball(filename)
	if err != nil {
		return PackedMetadata{}, err
	}
	if sha256.String() != expected.SHA256 || sha512.String() != expected.SHA512 {
		return PackedMetadata{}, npmValidationError("windlass.verify.error.digest-mismatch", "primary_artifact", "packed artifact differs from the verified handoff")
	}
	return packed, nil
}

func verifiedProducerManifest(metadata BuildMetadata, parameters ExternalParameters) (manifest, error) {
	if err := validateFinalDependencies(metadata.ResolvedDependencies, parameters); err != nil {
		return manifest{}, err
	}
	snapshot, err := metadata.ManifestEvidence.Decode(parameters.Package.Directory)
	if err != nil {
		return manifest{}, err
	}
	for _, descriptor := range metadata.ResolvedDependencies {
		if descriptor.Name == "package-manifest" && descriptor.Digest["sha256"] != digest.SumSHA256(snapshot.Content).String() {
			return manifest{}, npmValidationError(IDPackageManifestEvidence, "manifest_evidence", "manifest bytes disagree with the signed descriptor digest")
		}
	}
	decoded, err := decodeSourceManifestSnapshot(snapshot)
	if err != nil {
		return manifest{}, npmValidationError(IDPackageManifestEvidence, "manifest_evidence", "manifest bytes are invalid")
	}
	if decoded.Name == "" || decoded.Version == "" {
		return manifest{}, npmValidationError(IDPackageMetadataRequired, "manifest_evidence", "manifest requires name and version")
	}
	if decoded.Private {
		return manifest{}, npmValidationError(IDPackagePrivate, "manifest_evidence", "private packages are not publishable")
	}
	repository, err := normalizeRepository(decoded.Repository)
	if err != nil || repository != parameters.Workflow.Repository {
		return manifest{}, npmValidationError(IDPackageRepositoryIdentityMismatch, "manifest_evidence", "manifest repository differs from the caller repository")
	}
	publishConfig, err := decodePublishConfig(decoded.PublishConfig)
	if err != nil {
		return manifest{}, err
	}
	if _, err := ResolvePublishIntent(stringOrEmpty(parameters.Publish.InputRegistryURL), stringOrEmpty(parameters.Publish.InputDistTag), stringOrEmpty(parameters.Publish.InputAccess), publishConfig); err != nil {
		return manifest{}, err
	}
	if err := validateBuildDisposition(metadata.ManifestEvidence, parameters.Package.Directory, metadata.BuildScriptDisposition); err != nil {
		return manifest{}, err
	}
	return decoded, nil
}

func validateProducerBindings(input NPMProvenanceInput, parameters ExternalParameters) error {
	dependencies, err := finalDependencySet(input.BuildMetadata.ResolvedDependencies)
	if err != nil {
		return err
	}
	if identity.ValidateFullSHA(input.PeeledReleaseRevision) != nil || dependencies["built-source"].Digest["gitCommit"] != input.PeeledReleaseRevision {
		return npmValidationError(IDBuiltSource, "resolvedDependencies.built-source", "built commit differs from verified tag resolution")
	}
	if identity.ValidateFullSHA(input.InvocationRevision) != nil || dependencies["invocation-source"].Digest["gitCommit"] != input.InvocationRevision {
		return npmValidationError(IDInvocationSource, "resolvedDependencies.invocation-source", "invocation commit differs from authenticated invocation evidence")
	}
	decoded, err := verifiedProducerManifest(input.BuildMetadata, parameters)
	if err != nil {
		return err
	}
	if input.Packed.Name != decoded.Name || input.Packed.Version != decoded.Version {
		return npmValidationError(IDPackedPackageMetadataMismatch, "packed", "verified tarball identity disagrees with the original source manifest")
	}
	if err := rejectPackedCorepackEnvironment(input.Packed.Files); err != nil {
		return err
	}
	ref := effectiveBuiltRef(parameters)
	if ref != "refs/tags/v"+decoded.Version || ref != input.RuntimeReleaseRef {
		return npmValidationError(IDReleaseRefMismatch, "resolvedDependencies.built-source", "effective built ref differs from the version tag or verified runtime ref")
	}
	if input.GitHub.EventName != "push" && input.GitHub.EventName != "workflow_dispatch" {
		return npmValidationError(IDSourceRefInvalid, "internalParameters.github.event_name", "release event is unsupported")
	}
	if input.GitHub.EventName == "push" && parameters.Workflow.Ref != ref {
		return npmValidationError(IDSourceRefInvalid, "externalParameters.workflow.ref", "push invocation must identify the built release tag")
	}
	_, usesCorepack := dependencies["package-manager-distribution"]
	pair, err := toolchainpins.NodeRuntimePair()
	if err != nil {
		return err
	}
	if input.NodeJSVersion != "v"+pair.NodeVersion || input.NPMVersion != pair.NPMVersion || usesCorepack != (input.CorepackVersion != nil) {
		return npmValidationError("windlass.verify.error.builder-version-mismatch", "builder.version", "observed versions must match the pinned pair and acquisition path")
	}
	return nil
}
