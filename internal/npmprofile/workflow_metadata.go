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
)

// WorkflowBuildMetadataConfig contains the public npm-only inputs and trusted runtime observations.
type WorkflowBuildMetadataConfig struct {
	ArtifactName           string
	RegistryURLInput       string
	DistTagInput           string
	AccessInput            string
	ReleaseAssetMode       bool
	ReleaseTag             string
	ProvenanceSidecar      string
	LinkedArtifactMetadata bool
	EventName              string
	RefType                string
	Ref                    string
	Revision               string
	SourceRefInput         string
	InvocationRef          string
	InvocationRevision     string
	WorkflowSHA            string
	CallerWorkflowFilename string
	RegistryState          RegistryPreflightState
}

// FinalizeWorkflowBuildMetadata creates the complete closed metadata uploaded by the build job.
func FinalizeWorkflowBuildMetadata(selection Result, build BuildPackResult, config WorkflowBuildMetadataConfig) (BuildMetadata, error) {
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
	publish.PackageIdentityPreexisting = optionalBool(config.RegistryState.PackageExists)
	publish.PackageVersionPreexisting = optionalBool(config.RegistryState.VersionExists)
	packageURL, err := npmRegistryPackageURL(publish.ResolvedRegistryURL, selection.Package.Name, selection.Package.Version)
	if err != nil {
		return BuildMetadata{}, err
	}
	builderID, err := identity.NewBuilderID(NPMWorkflowPath, config.WorkflowSHA)
	if err != nil {
		return BuildMetadata{}, err
	}
	parameters := ExternalParameters{
		Source:   source,
		Workflow: WorkflowParameters{Path: NPMWorkflowPath, SHA: config.WorkflowSHA, BuilderID: builderID},
		Runtime:  RuntimeParameters{Runner: "ubuntu-24.04", NodeVersion: strings.TrimPrefix(build.Toolchain.NodeVersion, "v"), NPMVersion: build.Toolchain.NPMVersion},
		Package: PackageParameters{
			Directory: selection.Package.Directory, WorkspaceRoot: workspaceRoot(selection.Package), SourceManifest: packageManifestPath(selection.Package.Directory),
			Name: selection.Package.Name, Version: selection.Package.Version, Repository: selection.Package.Repository,
			TarballName: filepath.Base(build.TarballPath), PackageURL: packageURL, PackedName: build.Packed.Name,
			PackedVersion: build.Packed.Version, PackedFiles: append([]string(nil), build.Packed.Files...), ConsumerSurface: cloneRawMap(build.Packed.ConsumerSurface),
		},
		PackageManager: packageManagerParameters(selection, build),
		Publish:        publish,
		Release:        ReleaseParameters{Ref: source.Ref, VersionTag: strings.TrimPrefix(source.Ref, "refs/tags/")},
		Distribution:   DistributionParameters{},
		Caller:         CallerParameters{WorkflowFilename: config.CallerWorkflowFilename},
		Build:          BuildParameters{ScriptPresent: build.BuildScript.Present, ScriptResult: build.BuildScript.Result},
	}
	encoded, err := EncodeExternalParameters(parameters)
	if err != nil {
		return BuildMetadata{}, err
	}
	dependencies, err := workflowResolvedDependencies(selection, build, parameters)
	if err != nil {
		return BuildMetadata{}, err
	}
	if err := validateResolvedDependencies(dependencies, parameters); err != nil {
		return BuildMetadata{}, err
	}
	return BuildMetadata{
		SchemaVersion:      "1",
		PrimaryArtifact:    PrimaryArtifact{ArtifactName: config.ArtifactName, PayloadFileName: filepath.Base(build.TarballPath), SHA256: build.SHA256.String(), SHA512: build.SHA512.String()},
		ExternalParameters: encoded, ResolvedDependencies: dependencies,
	}, nil
}

// ResolveWorkflowPublishIntent reads and resolves the selected package's complete publish intent.
func ResolveWorkflowPublishIntent(selection Result, registryInput, distTagInput, accessInput string) (PublishParameters, error) {
	manifest, _, err := readManifest(filepath.Join(selection.Package.RealDirectory, "package.json"))
	if err != nil {
		return PublishParameters{}, fmt.Errorf("read selected package publish intent: %w", err)
	}
	publishConfig, err := decodePublishConfig(manifest.PublishConfig)
	if err != nil {
		return PublishParameters{}, err
	}
	return ResolvePublishIntent(registryInput, distTagInput, accessInput, publishConfig)
}

// ResolvePublishIntent applies caller input, source publishConfig, and npm defaults without silent conflict resolution.
func ResolvePublishIntent(registryInput, distTagInput, accessInput string, source *PublishConfigParameters) (PublishParameters, error) {
	registryInput = trimASCII(registryInput)
	distTagInput = trimASCII(distTagInput)
	accessInput = trimASCII(accessInput)
	if source != nil && source.Provenance != nil && !*source.Provenance {
		return PublishParameters{}, errors.New("publishConfig.provenance must not disable provenance")
	}
	registry, err := resolveStringIntent(registryInput, sourceValue(source, "registry"), "https://registry.npmjs.org/", "registry-url")
	if err != nil {
		return PublishParameters{}, err
	}
	normalizedRegistry, err := normalizeRegistryURL(registry)
	if err != nil {
		return PublishParameters{}, err
	}
	distTag, err := resolveStringIntent(distTagInput, sourceValue(source, "tag"), "latest", "dist-tag")
	if err != nil || !validDistTag(distTag) {
		return PublishParameters{}, errors.New("dist-tag is invalid or conflicts with publishConfig")
	}
	access, err := resolveStringIntent(accessInput, sourceValue(source, "access"), "", "access")
	if err != nil || access != "" && access != "public" && access != "restricted" {
		return PublishParameters{}, errors.New("access is invalid or conflicts with publishConfig")
	}
	result := PublishParameters{
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

func workflowSourceParameters(selection Result, config WorkflowBuildMetadataConfig) (SourceParameters, error) {
	config.SourceRefInput = NormalizeSourceRefInput(config.SourceRefInput)
	if err := ValidateSourceRefInput(config.SourceRefInput, config.InvocationRef, selection.Package.Version); err != nil {
		return SourceParameters{}, err
	}
	source := SourceParameters{
		Repository: selection.Package.Repository,
		Ref:        config.Ref,
		Revision:   config.Revision,
		EventName:  config.EventName,
		RefType:    config.RefType,
	}
	if config.SourceRefInput == "" {
		return source, nil
	}
	if config.InvocationRef == "" || config.InvocationRevision == "" {
		return SourceParameters{}, npmValidationError(IDUnexpectedExternalParameters, "source-ref", "source-ref requires the complete invocation ref and revision")
	}
	if config.Ref != config.SourceRefInput {
		return SourceParameters{}, sourceRefError("resolved built ref differs from source-ref input")
	}
	source.Ref = config.SourceRefInput
	source.Revision = config.Revision
	source.RefType = "tag"
	source.InputRef = optionalString(config.SourceRefInput)
	source.InvocationRef = optionalString(config.InvocationRef)
	source.InvocationRevision = optionalString(config.InvocationRevision)
	return source, nil
}

func validateWorkflowRuntime(selection Result, build BuildPackResult, config WorkflowBuildMetadataConfig, source SourceParameters) error {
	if config.ReleaseAssetMode || trimASCII(config.ReleaseTag) != "" || trimASCII(config.ProvenanceSidecar) != "" || config.LinkedArtifactMetadata {
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
	if config.ArtifactName == "" || selection.Package.Name != build.PackageName || selection.Package.Version != build.PackageVersion ||
		build.Packed.Name != build.PackageName || build.Packed.Version != build.PackageVersion || build.Toolchain.NodeVersion == "" || build.Toolchain.NPMVersion == "" {
		return errors.New("build result is incomplete or conflicts with selected package")
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

func workflowResolvedDependencies(selection Result, build BuildPackResult, parameters ExternalParameters) ([]provenance.ResourceDescriptor, error) {
	lockfilePath := selection.Manager.SelectedLockfilePath
	lockfileBytes, err := readBoundedRegularFile(filepath.Join(selection.Package.RealManagerRoot, filepath.Base(lockfilePath)), maxManifestSize)
	if err != nil {
		return nil, fmt.Errorf("read selected lockfile: %w", err)
	}
	dependencies := []provenance.ResourceDescriptor{{
		Name: "lockfile", URI: "git+" + parameters.Source.Repository + "@" + parameters.Source.Revision + "#" + lockfilePath,
		Digest: map[string]string{"sha256": digest.SumSHA256(lockfileBytes).String()},
		Annotations: annotations(map[string]any{
			"package_manager": selection.Manager.Name, "package_manager_root": selection.Package.ManagerRoot,
			"selection_source": selection.Manager.Source, "selection_manifest_path": nullableString(selection.Manager.SelectionManifestPath),
			"selection_lockfile_path": lockfilePath, "stale_non_selected_lockfiles": nonNilStrings(selection.Manager.IgnoredLockfilePaths),
		}),
	}}
	if selection.Manager.Name != ManagerNPM {
		distribution := build.Toolchain.Distribution
		if distribution == nil {
			return nil, errors.New("corepack package-manager distribution capture is required")
		}
		dependencies = append(dependencies, provenance.ResourceDescriptor{
			Name: "package-manager-distribution", URI: distribution.URL, Digest: map[string]string{"sha512": distribution.SHA512},
			Annotations: annotations(map[string]any{"digest_authority": distribution.DigestAuthority, "package_manager": distribution.PackageManager, "package_manager_version": distribution.PackageManagerVer, "acquisition_source": distribution.AcquisitionSource}),
		})
	}
	runner := build.Toolchain.Runner
	dependencies = append(dependencies, provenance.ResourceDescriptor{
		Name: "runner-image", URI: runner.IncludedSoftwareURL,
		Annotations: annotations(map[string]any{"image_os": runner.ImageOS, "image_version": runner.ImageVersion, "node_version": build.Toolchain.NodeVersion}),
	})
	return dependencies, nil
}

func packageManagerParameters(selection Result, build BuildPackResult) PackageManagerParameters {
	parameters := PackageManagerParameters{
		Name: selection.Manager.Name, Version: build.Toolchain.PackageManagerVersion, SelectionSource: selection.Manager.Source,
		SelectionManifest: optionalString(filepath.Base(selection.Manager.SelectionManifestPath)), SelectionManifestPath: optionalString(selection.Manager.SelectionManifestPath),
		SelectionLockfilePath: optionalString(selection.Manager.SelectionLockfilePath), Root: selection.Package.ManagerRoot,
		IgnoredLockfilePaths: append([]string(nil), selection.Manager.IgnoredLockfilePaths...),
	}
	if parameters.Name == ManagerYarn {
		parameters.YarnInstallMode = "immutable"
	}
	return parameters
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

func optionalBool(value bool) *bool {
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
