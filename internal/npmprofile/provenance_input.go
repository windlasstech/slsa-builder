package npmprofile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
	"github.com/windlasstech/slsa-builder/internal/digest"
	"github.com/windlasstech/slsa-builder/internal/identity"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

// EncodeExternalParameters validates and JCS-encodes the closed npm external parameter object.
func EncodeExternalParameters(parameters ExternalParameters) (json.RawMessage, error) {
	encoded, err := json.Marshal(parameters)
	if err != nil {
		return nil, npmValidationError(IDUnexpectedExternalParameters, "externalParameters", "encode external parameters")
	}
	canonical, err := canonicaljson.Canonicalize(encoded)
	if err != nil {
		return nil, npmValidationError(IDUnexpectedExternalParameters, "externalParameters", "canonicalize external parameters")
	}
	if err := validateExternalParameters(parameters, ""); err != nil {
		return nil, err
	}
	return canonical, nil
}

// DecodeExternalParameters strictly decodes the duplicate-free closed npm parameter object.
func DecodeExternalParameters(encoded []byte) (ExternalParameters, error) {
	if err := canonicaljson.Validate(encoded); err != nil {
		return ExternalParameters{}, npmValidationError(IDUnexpectedExternalParameters, "externalParameters", "external parameters must be duplicate-free JSON")
	}
	if err := validateExternalParameterShape(encoded); err != nil {
		return ExternalParameters{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var parameters ExternalParameters
	if err := decoder.Decode(&parameters); err != nil {
		return ExternalParameters{}, npmValidationError(IDUnexpectedExternalParameters, "externalParameters", "external parameters violate the closed schema")
	}
	if decoder.More() {
		return ExternalParameters{}, npmValidationError(IDUnexpectedExternalParameters, "externalParameters", "external parameters contain trailing data")
	}
	if err := validateExternalParameters(parameters, ""); err != nil {
		return ExternalParameters{}, err
	}
	return parameters, nil
}

// NPMPackagePURL constructs the ADR 0064 npm Package URL subject name.
func NPMPackagePURL(name, version string) (string, error) {
	if invalidPURLText(version) {
		return "", npmValidationError(IDNPMSubjectMismatch, "subject[0].name", "package version cannot form an npm PURL")
	}
	if strings.HasPrefix(name, "@") {
		remainder := strings.TrimPrefix(name, "@")
		if strings.Count(remainder, "/") != 1 {
			return "", npmValidationError(IDNPMSubjectMismatch, "subject[0].name", "scoped npm package name must contain one scope separator")
		}
		scope, packageName, _ := strings.Cut(remainder, "/")
		if invalidPURLText(scope) || invalidPURLText(packageName) {
			return "", npmValidationError(IDNPMSubjectMismatch, "subject[0].name", "scoped npm package name cannot form a PURL")
		}
		return "pkg:npm/%40" + percentEncode(scope) + "/" + percentEncode(packageName) + "@" + percentEncode(version), nil
	}
	if strings.Contains(name, "/") || invalidPURLText(name) {
		return "", npmValidationError(IDNPMSubjectMismatch, "subject[0].name", "unscoped npm package name cannot form a PURL")
	}
	return "pkg:npm/" + percentEncode(name) + "@" + percentEncode(version), nil
}

func invalidPURLText(value string) bool {
	if value == "" {
		return true
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f || character == '\\' {
			return true
		}
	}
	return false
}

// NewProvenanceSigningInput assembles and validates the exact npm custom-mode signing inputs.
func NewProvenanceSigningInput(input NPMProvenanceInput) (ProvenanceSigningInput, error) {
	parameters, err := DecodeExternalParameters(input.BuildMetadata.ExternalParameters)
	if err != nil {
		return ProvenanceSigningInput{}, err
	}
	if err := validateInputBindings(input, parameters); err != nil {
		return ProvenanceSigningInput{}, err
	}
	sha256Value, err := digest.ParseSHA256(input.BuildMetadata.PrimaryArtifact.SHA256)
	if err != nil {
		return ProvenanceSigningInput{}, npmValidationError(IDMissingSubjectSHA256, "buildMetadata.primary_artifact.sha256", "tarball SHA-256 must be lowercase hexadecimal")
	}
	sha512Value, err := digest.ParseSHA512(input.BuildMetadata.PrimaryArtifact.SHA512)
	if err != nil {
		return ProvenanceSigningInput{}, npmValidationError(IDMissingSubjectSHA512, "buildMetadata.primary_artifact.sha512", "tarball SHA-512 must be lowercase hexadecimal")
	}
	subjectName, err := NPMPackagePURL(parameters.Package.Name, parameters.Package.Version)
	if err != nil {
		return ProvenanceSigningInput{}, err
	}
	version := map[string]string{"nodejs": input.NodeJSVersion}
	if input.CorepackVersion != nil {
		version["corepack"] = *input.CorepackVersion
	}
	predicate := provenance.Predicate{
		BuildDefinition: provenance.BuildDefinition{
			BuildType:            NPMBuildType,
			ExternalParameters:   append(json.RawMessage(nil), input.BuildMetadata.ExternalParameters...),
			InternalParameters:   json.RawMessage(`{}`),
			ResolvedDependencies: cloneNPMDependencies(input.BuildMetadata.ResolvedDependencies),
		},
		RunDetails: provenance.RunDetails{
			Builder: provenance.Builder{
				ID:      input.BuilderID,
				Version: version,
				BuilderDependencies: []provenance.BuilderDependency{{
					URI:         "git+https://github.com/actions/attest@" + input.SigningAdapterSHA,
					Digest:      map[string]string{"gitCommit": input.SigningAdapterSHA},
					Annotations: map[string]string{"role": "signing-adapter"},
				}},
			},
			Metadata: provenance.Metadata{InvocationID: input.InvocationID, StartedOn: input.StartedOn, FinishedOn: input.FinishedOn},
		},
	}
	signing := ProvenanceSigningInput{
		Subject: provenance.Subject{
			Name:   subjectName,
			Digest: map[string]string{"sha256": sha256Value.String(), "sha512": sha512Value.String()},
		},
		PredicateType:     provenance.PredicateType,
		Predicate:         predicate,
		PredicateFileName: NPMProvenancePredicateFile,
	}
	if err := ValidateNPMStatement(signing.Statement(), input); err != nil {
		return ProvenanceSigningInput{}, err
	}
	signing.PredicateJSON, err = predicate.CanonicalJSON()
	if err != nil {
		return ProvenanceSigningInput{}, fmt.Errorf("canonicalize npm predicate: %w", err)
	}
	return signing, nil
}

func validateInputBindings(input NPMProvenanceInput, parameters ExternalParameters) error {
	if input.BuildMetadata.SchemaVersion != "1" {
		return npmValidationError(IDUnexpectedExternalParameters, "buildMetadata.schema_version", "build metadata schema version must be 1")
	}
	artifact := input.BuildMetadata.PrimaryArtifact
	if artifact.ArtifactName == "" || artifact.PayloadFileName == "" || strings.ContainsAny(artifact.PayloadFileName, `/\\`) || !strings.HasSuffix(artifact.PayloadFileName, ".tgz") || artifact.PayloadFileName != parameters.Package.TarballName {
		return npmValidationError(IDNPMSubjectMismatch, "buildMetadata.primary_artifact", "build metadata must identify the external-parameter tarball basename")
	}
	if input.BuilderID != parameters.Workflow.BuilderID {
		return npmValidationError("windlass.verify.error.builder-id-not-immutable", "builder.id", "builder identity differs from the signed workflow identity")
	}
	if err := identity.ValidateBuilderID(input.BuilderID); err != nil {
		return err
	}
	if err := ValidateReleaseRefEquality(parameters.Source.Ref, parameters.Release.Ref, input.RuntimeReleaseRef, parameters.Release.VersionTag, parameters.Source.Revision, input.PeeledReleaseRevision); err != nil {
		return err
	}
	if input.NodeJSVersion != "v"+parameters.Runtime.NodeVersion {
		return npmValidationError("windlass.verify.error.builder-version-mismatch", "builder.version.nodejs", "observed Node.js version differs from external parameters")
	}
	usesCorepack := parameters.PackageManager.Name != ManagerNPM
	if usesCorepack != (input.CorepackVersion != nil) {
		return npmValidationError("windlass.verify.error.builder-version-mismatch", "builder.version.corepack", "Corepack presence must match package-manager acquisition")
	}
	if err := identity.ValidateFullSHA(input.SigningAdapterSHA); err != nil {
		return npmValidationError(IDBuilderDependenciesMismatch, "builder.builderDependencies", "signing adapter revision must be a full lowercase SHA")
	}
	validator := npmProfileValidator{parameters: parameters, encodedParameters: input.BuildMetadata.ExternalParameters, dependencies: input.BuildMetadata.ResolvedDependencies, sha256: input.BuildMetadata.PrimaryArtifact.SHA256, sha512: input.BuildMetadata.PrimaryArtifact.SHA512}
	return validator.ValidateResolvedDependencies(cloneNPMDependencies(input.BuildMetadata.ResolvedDependencies))
}

// ValidateNPMStatement checks an emitted Statement against the exact pre-sign npm input.
func ValidateNPMStatement(statement provenance.Statement, input NPMProvenanceInput) error {
	parameters, err := DecodeExternalParameters(input.BuildMetadata.ExternalParameters)
	if err != nil {
		return err
	}
	if statement.Predicate.BuildDefinition.BuildType != NPMBuildType {
		return npmValidationError("windlass.verify.error.wrong-build-type", "predicate.buildDefinition.buildType", "npm build type differs from the acquired-domain v1 identifier")
	}
	if statement.Predicate.RunDetails.Builder.ID != input.BuilderID {
		return npmValidationError("windlass.verify.error.wrong-builder-id", "predicate.runDetails.builder.id", "builder ID differs from the signing input")
	}
	metadata := statement.Predicate.RunDetails.Metadata
	if metadata.InvocationID != input.InvocationID || metadata.StartedOn != input.StartedOn || metadata.FinishedOn != input.FinishedOn {
		return npmValidationError("windlass.verify.error.statement-assembly-mismatch", "predicate.runDetails.metadata", "signed metadata differs from the pre-sign candidate")
	}
	validator := npmProfileValidator{parameters: parameters, encodedParameters: input.BuildMetadata.ExternalParameters, dependencies: input.BuildMetadata.ResolvedDependencies, sha256: input.BuildMetadata.PrimaryArtifact.SHA256, sha512: input.BuildMetadata.PrimaryArtifact.SHA512}
	expectations := provenance.Expectations{
		SourceRepositoryURI: parameters.Source.Repository,
		Builder: provenance.BuilderExpectations{
			NodeJSVersion:     input.NodeJSVersion,
			CorepackVersion:   input.CorepackVersion,
			SigningAdapterSHA: input.SigningAdapterSHA,
		},
	}
	_, err = statement.Validate(expectations, validator)
	return err
}

// ValidateReleaseRefEquality proves full-tag equality and the terminal peeled commit binding.
func ValidateReleaseRefEquality(sourceRef, releaseRef, runtimeRef, versionTag, sourceRevision, peeledRevision string) error {
	for _, ref := range []string{sourceRef, releaseRef, runtimeRef} {
		if identity.ValidateReleaseRef(ref) != nil {
			return npmValidationError(IDReleaseRefMismatch, "externalParameters.release.ref", "release identity must use a full Git tag ref")
		}
	}
	if sourceRef != releaseRef || sourceRef != runtimeRef || strings.TrimPrefix(sourceRef, "refs/tags/") != versionTag {
		return npmValidationError(IDReleaseRefMismatch, "externalParameters.release", "source, release, runtime, and version-tag identities must be equal")
	}
	if identity.ValidateFullSHA(sourceRevision) != nil || identity.ValidateFullSHA(peeledRevision) != nil || sourceRevision != peeledRevision {
		return npmValidationError(IDReleaseRefMismatch, "externalParameters.source.revision", "release tag must peel to the signed source revision")
	}
	return nil
}

func npmPURL(name, version string) string {
	value, err := NPMPackagePURL(name, version)
	if err != nil {
		return ""
	}
	return value
}

func cloneNPMDependencies(values []provenance.ResourceDescriptor) []provenance.ResourceDescriptor {
	cloned := make([]provenance.ResourceDescriptor, len(values))
	for index, value := range values {
		cloned[index] = provenance.ResourceDescriptor{Name: value.Name, URI: value.URI, Digest: cloneNPMStringMap(value.Digest), Annotations: make(map[string]json.RawMessage, len(value.Annotations))}
		for name, annotation := range value.Annotations {
			cloned[index].Annotations[name] = append(json.RawMessage(nil), annotation...)
		}
	}
	return cloned
}

func cloneNPMStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for name, value := range values {
		cloned[name] = value
	}
	return cloned
}
