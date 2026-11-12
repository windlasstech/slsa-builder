package npmprofile

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/windlasstech/slsa-builder/internal/digest"
	"github.com/windlasstech/slsa-builder/internal/identity"
	"github.com/windlasstech/slsa-builder/internal/provenance"
	"github.com/windlasstech/slsa-builder/internal/toolchainpins"
)

// WorkflowBuildMetadataConfig contains the public npm-only inputs and trusted runtime observations.
type WorkflowBuildMetadataConfig struct {
	StartedOn              string
	FinishedOn             string
	ArtifactName           string
	RegistryURLInput       string
	DistTagInput           string
	AccessInput            string
	ReleaseAssetMode       bool
	LinkedArtifactMetadata bool
	EventName              string
	RefType                string
	Ref                    string
	Revision               string
	SourceRefInput         string
	InvocationRef          string
	InvocationRevision     string
	WorkflowSHA            string
	CallerWorkflowPath     string
	CallerWorkflowFilename string
	Repository             string
	RegistryState          RegistryPreflightState
}

// FinalizeWorkflowBuildMetadata creates the complete closed metadata uploaded by the build job.
func FinalizeWorkflowBuildMetadata(selection Result, build BuildPackResult, config WorkflowBuildMetadataConfig) (BuildMetadata, error) {
	if config.InvocationRef == "" {
		config.InvocationRef = config.Ref
	}
	source, err := workflowSourceParameters(selection, config)
	if err != nil {
		return BuildMetadata{}, err
	}
	if err := validateWorkflowRuntime(selection, build, config, source); err != nil {
		return BuildMetadata{}, err
	}
	publish, err := ResolveWorkflowPublishIntent(selection, config.RegistryURLInput, config.DistTagInput, config.AccessInput)
	if err != nil {
		return BuildMetadata{}, err
	}
	evidence, err := captureManifestEvidence(selection.Package.SourceManifest, selection.Package.Directory)
	if err != nil {
		return BuildMetadata{}, err
	}
	if err := validateBuildDisposition(evidence, selection.Package.Directory, build.BuildScript.Result); err != nil {
		return BuildMetadata{}, err
	}
	if build.BuildScript.Present != (build.BuildScript.Result == BuildScriptExecuted) {
		return BuildMetadata{}, npmValidationError(IDBuildScriptDisposition, "build_script_disposition", "captured script presence and disposition disagree")
	}
	parameters := ExternalParameters{
		Source:       SourceParameters{Ref: config.SourceRefInput},
		Workflow:     WorkflowParameters{Ref: config.InvocationRef, Repository: config.Repository, Path: config.CallerWorkflowPath},
		Package:      PackageParameters{Directory: selection.Package.Directory},
		Publish:      PublishParameters{InputRegistryURL: rawInput(config.RegistryURLInput), InputDistTag: rawInput(config.DistTagInput), InputAccess: rawInput(config.AccessInput)},
		Distribution: DistributionParameters{},
	}
	if publish.ResolvedRegistryURL == "https://registry.npmjs.org/" && (!config.RegistryState.PackageExists || config.RegistryState.VersionExists) {
		return BuildMetadata{}, npmValidationError(IDUnexpectedExternalParameters, "registry-preflight", "npmjs requires an existing package identity and absent version")
	}
	encoded, err := EncodeExternalParameters(parameters)
	if err != nil {
		return BuildMetadata{}, err
	}
	dependencies, err := workflowResolvedDependencies(selection, build, source)
	if err != nil {
		return BuildMetadata{}, err
	}
	dependencies, err = prependSourceDependencies(dependencies, selection, config)
	if err != nil {
		return BuildMetadata{}, err
	}
	if err := validateFinalDependencies(dependencies, parameters); err != nil {
		return BuildMetadata{}, err
	}
	return BuildMetadata{
		StartedOn: config.StartedOn, FinishedOn: config.FinishedOn,
		SchemaVersion:      "1",
		PrimaryArtifact:    PrimaryArtifact{ArtifactName: config.ArtifactName, PayloadFileName: filepath.Base(build.TarballPath), SHA256: build.SHA256.String(), SHA512: build.SHA512.String()},
		ExternalParameters: encoded, ResolvedDependencies: dependencies,
		ManifestEvidence: evidence, BuildScriptDisposition: build.BuildScript.Result,
	}, nil
}

// ResolveWorkflowPublishIntent reads and resolves the selected package's complete publish intent.
func ResolveWorkflowPublishIntent(selection Result, registryInput, distTagInput, accessInput string) (PublishIntent, error) {
	manifest, err := decodeSourceManifestSnapshot(selection.Package.SourceManifest)
	if err != nil {
		return PublishIntent{}, fmt.Errorf("read selected package publish intent: %w", err)
	}
	publishConfig, err := decodePublishConfig(manifest.PublishConfig)
	if err != nil {
		return PublishIntent{}, err
	}
	return ResolvePublishIntent(registryInput, distTagInput, accessInput, publishConfig)
}

// ResolvePublishIntent applies caller input, source publishConfig, and npm defaults without silent conflict resolution.
func ResolvePublishIntent(registryInput, distTagInput, accessInput string, source *PublishConfigParameters) (PublishIntent, error) {
	registryInput = trimASCII(registryInput)
	distTagInput = trimASCII(distTagInput)
	accessInput = trimASCII(accessInput)
	if source != nil && source.Provenance != nil && !*source.Provenance {
		return PublishIntent{}, errors.New("publishConfig.provenance must not disable provenance")
	}
	registryCaller, registrySource := registryInput, trimASCII(sourceValue(source, "registry"))
	if registryCaller != "" {
		normalized, err := normalizeRegistryURL(registryCaller)
		if err != nil {
			return PublishIntent{}, err
		}
		registryCaller = normalized.String()
	}
	if registrySource != "" {
		normalized, err := normalizeRegistryURL(registrySource)
		if err != nil {
			return PublishIntent{}, err
		}
		registrySource = normalized.String()
	}
	registry, err := resolveStringIntent(registryCaller, registrySource, "https://registry.npmjs.org/", "registry-url")
	if err != nil {
		return PublishIntent{}, err
	}
	normalizedRegistry, err := normalizeRegistryURL(registry)
	if err != nil {
		return PublishIntent{}, err
	}
	distTag, err := resolveStringIntent(distTagInput, trimASCII(sourceValue(source, "tag")), "latest", "dist-tag")
	if err != nil || !validDistTag(distTag) {
		return PublishIntent{}, errors.New("dist-tag is invalid or conflicts with publishConfig")
	}
	access, err := resolveStringIntent(accessInput, trimASCII(sourceValue(source, "access")), "", "access")
	if err != nil || access != "" && access != "public" && access != "restricted" {
		return PublishIntent{}, errors.New("access is invalid or conflicts with publishConfig")
	}
	result := PublishIntent{
		InputRegistryURL: optionalString(registryInput), InputDistTag: optionalString(distTagInput), InputAccess: optionalString(accessInput),
		PublishConfig: source, ResolvedRegistryURL: normalizedRegistry.String(), ResolvedDistTag: distTag,
		PublishAccessOption: optionalString(access), EffectiveAccess: access, TrustedPublishing: true, ProvenanceFile: true,
	}
	if result.EffectiveAccess == "" {
		result.EffectiveAccess = "existing-package-access"
	}
	if result.ResolvedRegistryURL != "https://registry.npmjs.org/" {
		result.CustomRegistrySupport = "unsupported-but-not-blocked"
	}
	return result, nil
}

type resolvedWorkflowSource struct {
	Repository string
	Ref        string
	Revision   string
	RefType    string
}

func workflowSourceParameters(selection Result, config WorkflowBuildMetadataConfig) (resolvedWorkflowSource, error) {
	config.SourceRefInput = NormalizeSourceRefInput(config.SourceRefInput)
	if err := ValidateSourceRefInput(config.SourceRefInput, config.InvocationRef, selection.Package.Version); err != nil {
		return resolvedWorkflowSource{}, err
	}
	if err := identity.ValidateCanonicalRepositoryURI(config.Repository); err != nil {
		return resolvedWorkflowSource{}, npmValidationError(IDUnexpectedExternalParameters, "observed-repository", "observed caller repository must be canonical")
	}
	if config.Repository != selection.Package.Repository {
		return resolvedWorkflowSource{}, npmValidationError("windlass.verify.error.package-repository-identity-mismatch", "observed-repository", "observed caller repository differs from the normalized manifest repository")
	}
	source := resolvedWorkflowSource{
		Repository: config.Repository,
		Ref:        config.Ref,
		Revision:   config.Revision,
		RefType:    config.RefType,
	}
	if config.SourceRefInput == "" {
		return source, nil
	}
	if config.InvocationRef == "" || config.InvocationRevision == "" {
		return resolvedWorkflowSource{}, npmValidationError(IDUnexpectedExternalParameters, "source-ref", "source-ref requires the complete invocation ref and revision")
	}
	if config.Ref != config.SourceRefInput {
		return resolvedWorkflowSource{}, sourceRefError("resolved built ref differs from source-ref input")
	}
	source.Ref = config.SourceRefInput
	source.Revision = config.Revision
	source.RefType = "tag"
	return source, nil
}

func validateWorkflowRuntime(selection Result, build BuildPackResult, config WorkflowBuildMetadataConfig, source resolvedWorkflowSource) error {
	if config.ReleaseAssetMode || config.LinkedArtifactMetadata {
		return errors.New("release-asset-only inputs require the later release-asset mode implementation")
	}
	if config.EventName != "push" && config.EventName != "workflow_dispatch" {
		return errors.New("release event is unsupported")
	}
	if source.RefType != "tag" || source.Ref != "refs/tags/v"+selection.Package.Version {
		return errors.New("release ref must be the package version tag")
	}
	if identity.ValidateFullSHA(source.Revision) != nil || identity.ValidateFullSHA(config.WorkflowSHA) != nil {
		return errors.New("source and reusable workflow revisions must be immutable full SHAs")
	}
	if config.CallerWorkflowFilename == "" || filepath.Base(config.CallerWorkflowFilename) != config.CallerWorkflowFilename ||
		!strings.HasSuffix(config.CallerWorkflowFilename, ".yml") && !strings.HasSuffix(config.CallerWorkflowFilename, ".yaml") {
		return errors.New("caller workflow filename is invalid")
	}
	if !validCallerWorkflowPath(config.CallerWorkflowPath) || config.CallerWorkflowPath == NPMWorkflowPath || filepath.Base(config.CallerWorkflowPath) != config.CallerWorkflowFilename {
		return errors.New("caller workflow path is invalid or conflicts with the caller filename")
	}
	if config.ArtifactName == "" || selection.Package.Name != build.PackageName || selection.Package.Version != build.PackageVersion ||
		build.Packed.Name != build.PackageName || build.Packed.Version != build.PackageVersion || build.Toolchain.NodeVersion == "" || build.Toolchain.NPMVersion == "" {
		return errors.New("build result is incomplete or conflicts with selected package")
	}
	if !minimumVersion(strings.TrimPrefix(build.Toolchain.NodeVersion, "v"), 24, 0, 0) || !minimumVersion(build.Toolchain.NPMVersion, 11, 5, 1) {
		return npmValidationError("windlass.verify.error.builder-version-mismatch", "builder.version", "captured toolchain does not satisfy the runtime floors")
	}
	if _, err := NPMPackagePURL(build.Packed.Name, build.Packed.Version); err != nil {
		return err
	}
	return nil
}

func decodePublishConfig(raw jsonRaw) (*PublishConfigParameters, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return nil, errors.New("publishConfig must be an object")
	}
	if _, forbidden := object["directory"]; forbidden {
		return nil, errors.New("publishConfig.directory is unsupported")
	}
	var result PublishConfigParameters
	for name, encoded := range object {
		switch name {
		case "registry":
			if json.Unmarshal(encoded, &result.Registry) != nil {
				return nil, errors.New("publishConfig.registry must be a string")
			}
		case "tag":
			if json.Unmarshal(encoded, &result.Tag) != nil {
				return nil, errors.New("publishConfig.tag must be a string")
			}
		case "access":
			if json.Unmarshal(encoded, &result.Access) != nil {
				return nil, errors.New("publishConfig.access must be a string")
			}
		case "provenance":
			var value bool
			if json.Unmarshal(encoded, &value) != nil {
				return nil, errors.New("publishConfig.provenance must be boolean")
			}
			result.Provenance = &value
		}
	}
	return &result, nil
}

func workflowResolvedDependencies(selection Result, build BuildPackResult, source resolvedWorkflowSource) ([]provenance.ResourceDescriptor, error) {
	lockfilePath := selection.Manager.SelectedLockfilePath
	lockfileBytes, err := readBoundedRegularFile(filepath.Join(selection.Package.RealManagerRoot, filepath.Base(lockfilePath)), maxManifestSize)
	if err != nil {
		return nil, fmt.Errorf("read selected lockfile: %w", err)
	}
	dependencies := []provenance.ResourceDescriptor{{
		Name: "lockfile", URI: "git+" + source.Repository + "@" + source.Revision + "#" + escapeLockfileFragment(lockfilePath),
		Digest: map[string]string{"sha256": digest.SumSHA256(lockfileBytes).String()}, MediaType: lockfileMediaType(selection.Manager.Name, lockfilePath),
		Annotations: annotations(map[string]any{
			"package_manager": selection.Manager.Name, "package_manager_root": selection.Package.ManagerRoot,
			"selection_source": selection.Manager.Source, "selection_manifest_path": nullableString(selection.Manager.SelectionManifestPath),
			"selection_lockfile_path": lockfilePath, "stale_non_selected_lockfiles": nonNilStrings(selection.Manager.IgnoredLockfilePaths),
		}),
	}}
	runtime, err := nodeRuntimeDescriptor()
	if err != nil {
		return nil, err
	}
	dependencies = append(dependencies, runtime)
	if selection.Manager.Name != ManagerNPM {
		distribution := build.Toolchain.Distribution
		if distribution == nil {
			return nil, errors.New("corepack package-manager distribution capture is required")
		}
		if distribution.PackageManager != selection.Manager.Name || distribution.PackageManagerVer != selection.Manager.Descriptor() || build.Toolchain.PackageManagerVersion != selection.Manager.Version {
			return nil, npmValidationError(IDResolvedDependenciesDistribution, "resolvedDependencies.package-manager-distribution", "captured and executed distribution must match the selected manager and version")
		}
		dependencies = append(dependencies, provenance.ResourceDescriptor{
			Name: "package-manager-distribution", URI: distribution.URL, Digest: map[string]string{"sha512": distribution.SHA512},
			MediaType:   distributionMediaType(*distribution),
			Annotations: annotations(map[string]any{"digest_authority": distribution.DigestAuthority, "package_manager": distribution.PackageManager, "package_manager_version": distribution.PackageManagerVer, "acquisition_source": distribution.AcquisitionSource}),
		})
	}
	runner := build.Toolchain.Runner
	dependencies = append(dependencies, provenance.ResourceDescriptor{
		Name: "runner-image", URI: runner.IncludedSoftwareURL,
		Annotations: annotations(map[string]any{"deployment_version": nullableString(runner.DeploymentVersion), "image_os": runner.ImageOS, "image_version": runner.ImageVersion}),
	})
	return dependencies, nil
}

func nodeRuntimeDescriptor() (provenance.ResourceDescriptor, error) {
	pair, err := toolchainpins.NodeRuntimePair()
	if err != nil {
		return provenance.ResourceDescriptor{}, npmValidationError("windlass.verify.error.input-unavailable", "resolvedDependencies.node-runtime", "embedded Node.js runtime pin is unavailable")
	}
	artifact, present := pair.Artifacts["linux-x64"]
	if !present {
		return provenance.ResourceDescriptor{}, npmValidationError("windlass.verify.error.input-unavailable", "resolvedDependencies.node-runtime", "embedded linux-x64 runtime artifact is unavailable")
	}
	return provenance.ResourceDescriptor{
		Name: "node-runtime", URI: artifact.URL, MediaType: "application/gzip",
		Digest: map[string]string{"sha256": artifact.SHA256},
		Annotations: map[string]json.RawMessage{
			"digest_authority": json.RawMessage(`"builder-pinned-record"`),
			"platform":         json.RawMessage(`"linux-x64"`),
		},
	}, nil
}

func workspaceRoot(pkg Package) *string {
	if pkg.ManagerRoot == pkg.Directory {
		return nil
	}
	return optionalString(pkg.ManagerRoot)
}

func packageManifestPath(directory string) string {
	if directory == "." {
		return "package.json"
	}
	return strings.TrimSuffix(directory, "/") + "/package.json"
}

func resolveStringIntent(input, source, fallback, name string) (string, error) {
	if input != "" && source != "" && input != source {
		return "", fmt.Errorf("%s conflicts with publishConfig", name)
	}
	if input != "" {
		return input, nil
	}
	if source != "" {
		return source, nil
	}
	return fallback, nil
}

func sourceValue(source *PublishConfigParameters, field string) string {
	if source == nil {
		return ""
	}
	switch field {
	case "registry":
		return source.Registry
	case "tag":
		return source.Tag
	case "access":
		return source.Access
	}
	return ""
}

func validDistTag(value string) bool {
	return value != "" && !strings.ContainsAny(value, " /\\\x00\t\r\n") && value != "." && value != ".."
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	copy := value
	return &copy
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func trimASCII(value string) string { return strings.Trim(value, " \t\r\n\f\v") }

func annotations(values map[string]any) map[string]json.RawMessage {
	result := make(map[string]json.RawMessage, len(values))
	for name, value := range values {
		encoded, err := json.Marshal(value)
		if err != nil {
			panic(err)
		}
		result[name] = encoded
	}
	return result
}

func cloneRawMap(values map[string]json.RawMessage) map[string]json.RawMessage {
	if values == nil {
		return nil
	}
	result := make(map[string]json.RawMessage, len(values))
	for name, value := range values {
		result[name] = append(json.RawMessage(nil), value...)
	}
	return result
}
