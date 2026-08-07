package npmprofile

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
	"github.com/windlasstech/slsa-builder/internal/diagnostic"
	"github.com/windlasstech/slsa-builder/internal/identity"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

var exactSemverPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

type npmProfileValidator struct {
	parameters        ExternalParameters
	encodedParameters json.RawMessage
	dependencies      []provenance.ResourceDescriptor
	sha256            string
	sha512            string
}

func (validator npmProfileValidator) ValidateSubject(subject provenance.Subject) error {
	want, err := NPMPackagePURL(validator.parameters.Package.Name, validator.parameters.Package.Version)
	if err != nil || subject.Name != want {
		return npmValidationError(IDNPMSubjectMismatch, "subject[0].name", "subject name must equal the npm package PURL")
	}
	if len(subject.Digest) != 2 {
		return npmValidationError(IDSubjectDigestScopeInvalid, "subject[0].digest", "subject digest must contain exactly sha256 and sha512")
	}
	sha256Value, present := subject.Digest["sha256"]
	if !present || digestEncodingInvalid(sha256Value, 64) {
		return npmValidationError(IDMissingSubjectSHA256, "subject[0].digest.sha256", "subject SHA-256 is required as lowercase hexadecimal")
	}
	sha512Value, present := subject.Digest["sha512"]
	if !present || digestEncodingInvalid(sha512Value, 128) {
		return npmValidationError(IDMissingSubjectSHA512, "subject[0].digest.sha512", "subject SHA-512 is required as lowercase hexadecimal")
	}
	if validator.sha256 != "" && sha256Value != validator.sha256 || validator.sha512 != "" && sha512Value != validator.sha512 {
		return npmValidationError("windlass.verify.error.digest-mismatch", "subject[0].digest", "subject digests differ from the verified packed artifact")
	}
	return nil
}

func validateExternalParameterShape(encoded []byte) error {
	var top map[string]json.RawMessage
	if json.Unmarshal(encoded, &top) != nil {
		return npmValidationError(IDUnexpectedExternalParameters, "externalParameters", "external parameters must be an object")
	}
	requiredTop := []string{"source", "workflow", "runtime", "package", "package_manager", "publish", "release", "distribution", "caller", "build"}
	if !exactObjectKeys(top, requiredTop, nil) {
		return npmValidationError(IDUnexpectedExternalParameters, "externalParameters", "external parameter groups are missing or unexpected")
	}
	shapes := []struct {
		name     string
		required []string
		optional []string
	}{
		{name: "source", required: []string{"repository", "ref", "revision", "event_name", "ref_type"}},
		{name: "workflow", required: []string{"path", "sha", "builder_id"}},
		{name: "runtime", required: []string{"runner", "node_version", "npm_version"}},
		{name: "package", required: []string{"directory", "workspace_root", "source_manifest", "name", "version", "private", "repository", "tarball_name", "package_url", "packed_name", "packed_version"}, optional: []string{"publish_config_raw", "packed_files", "consumer_surface"}},
		{name: "package_manager", required: []string{"name", "version", "selection_source", "selection_manifest", "selection_manifest_path", "selection_lockfile_path", "root"}, optional: []string{"ignored_lockfile_paths", "yarn_install_mode"}},
		{name: "publish", required: []string{"input_registry_url", "input_dist_tag", "input_access", "publish_config", "resolved_registry_url", "resolved_dist_tag", "publish_access_option", "effective_access", "trusted_publishing", "provenance_file", "package_identity_preexisting", "package_version_preexisting"}, optional: []string{"custom_registry_support"}},
		{name: "release", required: []string{"ref", "version_tag"}},
		{name: "distribution", required: []string{"release_asset_mode", "release_tag_supplied", "provenance_sidecar", "linked_artifact_metadata"}},
		{name: "caller", required: []string{"workflow_filename"}},
		{name: "build", required: []string{"script_present", "script_result"}},
	}
	for _, shape := range shapes {
		var object map[string]json.RawMessage
		if json.Unmarshal(top[shape.name], &object) != nil || !exactObjectKeys(object, shape.required, shape.optional) {
			return npmValidationError(IDUnexpectedExternalParameters, "externalParameters."+shape.name, "nested external parameter members are missing or unexpected")
		}
	}
	var packageObject map[string]json.RawMessage
	if err := json.Unmarshal(top["package"], &packageObject); err != nil {
		return npmValidationError(IDUnexpectedExternalParameters, "externalParameters.package", "package parameters must be an object")
	}
	if raw, present := packageObject["consumer_surface"]; present {
		var surface map[string]json.RawMessage
		allowed := []string{"exports", "main", "type", "bin", "types", "typings", "typesVersions", "files"}
		if json.Unmarshal(raw, &surface) != nil || !exactObjectKeys(surface, nil, allowed) {
			return npmValidationError(IDUnexpectedExternalParameters, "externalParameters.package.consumer_surface", "consumer surface contains an unknown member")
		}
	}
	var publishObject map[string]json.RawMessage
	if err := json.Unmarshal(top["publish"], &publishObject); err != nil {
		return npmValidationError(IDUnexpectedExternalParameters, "externalParameters.publish", "publish parameters must be an object")
	}
	if raw := publishObject["publish_config"]; string(raw) != "null" {
		var publishConfig map[string]json.RawMessage
		if json.Unmarshal(raw, &publishConfig) != nil || !exactObjectKeys(publishConfig, nil, []string{"registry", "access", "tag", "provenance"}) {
			return npmValidationError(IDUnexpectedExternalParameters, "externalParameters.publish.publish_config", "publishConfig contains an unknown member")
		}
	}
	return nil
}

func exactObjectKeys(object map[string]json.RawMessage, required, optional []string) bool {
	if object == nil || len(object) < len(required) || len(object) > len(required)+len(optional) {
		return false
	}
	allowed := make(map[string]bool, len(required)+len(optional))
	for _, name := range required {
		allowed[name] = true
		if _, present := object[name]; !present {
			return false
		}
	}
	for _, name := range optional {
		allowed[name] = true
	}
	for name := range object {
		if !allowed[name] {
			return false
		}
	}
	return true
}

func (validator npmProfileValidator) ValidateExternalParameters(encoded json.RawMessage) error {
	parameters, err := DecodeExternalParameters(encoded)
	if err != nil {
		return err
	}
	if err := validateExternalParameters(parameters, validator.parameters.Source.Repository); err != nil {
		return err
	}
	equal, err := canonicaljson.Equal(encoded, validator.encodedParameters)
	if err != nil || !equal {
		return npmValidationError(IDUnexpectedExternalParameters, "predicate.buildDefinition.externalParameters", "signed external parameters differ from the verified build metadata")
	}
	return nil
}

func (validator npmProfileValidator) ValidateResolvedDependencies(values []provenance.ResourceDescriptor) error {
	if err := validateResolvedDependencies(values, validator.parameters); err != nil {
		return err
	}
	wantByName := make(map[string]provenance.ResourceDescriptor, len(validator.dependencies))
	for _, dependency := range validator.dependencies {
		wantByName[dependency.Name] = dependency
	}
	for _, dependency := range values {
		want, present := wantByName[dependency.Name]
		if !present {
			return npmValidationError(IDResolvedDependenciesUnexpectedEntry, "predicate.buildDefinition.resolvedDependencies", "dependency name is not enumerated")
		}
		equal, err := equalJSONValues(dependency, want)
		if err != nil || !equal {
			return dependencyMismatch(dependency.Name)
		}
	}
	return nil
}

func validateExternalParameters(parameters ExternalParameters, expectedRepository string) error {
	if err := identity.ValidateCanonicalRepositoryURI(parameters.Source.Repository); err != nil {
		return npmValidationError(IDUnexpectedExternalParameters, "externalParameters.source.repository", "source repository must be canonical")
	}
	if expectedRepository != "" && parameters.Source.Repository != expectedRepository {
		return npmValidationError(IDUnexpectedExternalParameters, "externalParameters.source.repository", "source repository differs from the expected repository")
	}
	if parameters.Package.Repository != parameters.Source.Repository {
		return npmValidationError("windlass.verify.error.package-repository-identity-mismatch", "externalParameters.package.repository", "package and source repositories must match")
	}
	if identity.ValidateFullSHA(parameters.Source.Revision) != nil || identity.ValidateFullSHA(parameters.Workflow.SHA) != nil {
		return npmValidationError(IDUnexpectedExternalParameters, "externalParameters.source.revision", "source and workflow revisions must be full lowercase SHAs")
	}
	if parameters.Workflow.Path != NPMWorkflowPath {
		return npmValidationError(IDUnexpectedExternalParameters, "externalParameters.workflow.path", "workflow path must equal the npm producer entrypoint")
	}
	wantBuilderID, err := identity.NewBuilderID(parameters.Workflow.Path, parameters.Workflow.SHA)
	if err != nil || parameters.Workflow.BuilderID != wantBuilderID {
		return npmValidationError("windlass.verify.error.builder-id-not-immutable", "externalParameters.workflow.builder_id", "builder identity must bind the workflow path and SHA")
	}
	if parameters.Source.EventName == "" || parameters.Source.RefType != "tag" {
		return npmValidationError(IDUnexpectedExternalParameters, "externalParameters.source", "release source event and tag type are required")
	}
	if parameters.Runtime.Runner != "ubuntu-24.04" || !minimumVersion(parameters.Runtime.NodeVersion, 24, 0, 0) || !minimumVersion(parameters.Runtime.NPMVersion, 11, 5, 1) {
		return npmValidationError(IDUnexpectedExternalParameters, "externalParameters.runtime", "runtime must use ubuntu-24.04, Node.js 24+, and npm 11.5.1+")
	}
	if err := validatePackageParameters(parameters); err != nil {
		return err
	}
	if err := validatePackageManagerParameters(parameters.PackageManager); err != nil {
		return err
	}
	if err := validatePublishParameters(parameters.Publish); err != nil {
		return err
	}
	if parameters.Release.Ref == "" || parameters.Release.VersionTag == "" {
		return npmValidationError(IDUnexpectedExternalParameters, "externalParameters.release", "release ref and version tag are required")
	}
	if parameters.Distribution.ReleaseAssetMode {
		if parameters.Distribution.ProvenanceSidecar == nil || *parameters.Distribution.ProvenanceSidecar != "required" {
			return npmValidationError("windlass.verify.error.release-asset-mode-schema-error", "externalParameters.distribution.provenance_sidecar", "release mode requires the provenance sidecar")
		}
	} else if parameters.Distribution.ProvenanceSidecar != nil {
		return npmValidationError("windlass.verify.error.release-asset-mode-schema-error", "externalParameters.distribution.provenance_sidecar", "npm-only mode requires a null provenance sidecar")
	}
	if parameters.Caller.WorkflowFilename == "" || path.Base(parameters.Caller.WorkflowFilename) != parameters.Caller.WorkflowFilename || !strings.HasSuffix(parameters.Caller.WorkflowFilename, ".yml") {
		return npmValidationError("windlass.verify.error.trusted-publisher-mismatch", "externalParameters.caller.workflow_filename", "caller workflow filename must be an observed yml basename")
	}
	if parameters.Build.ScriptPresent != (parameters.Build.ScriptResult == BuildScriptExecuted) && parameters.Build.ScriptResult != BuildScriptSkippedAbsent {
		return npmValidationError(IDUnexpectedExternalParameters, "externalParameters.build", "build script presence and result disagree")
	}
	if parameters.Build.ScriptPresent && parameters.Build.ScriptResult != BuildScriptExecuted || !parameters.Build.ScriptPresent && parameters.Build.ScriptResult != BuildScriptSkippedAbsent {
		return npmValidationError(IDUnexpectedExternalParameters, "externalParameters.build", "build result must match script presence")
	}
	return validateOptionalExternalFields(parameters)
}

func validatePackageParameters(parameters ExternalParameters) error {
	packageParameters := parameters.Package
	if packageParameters.Directory == "" || packageParameters.SourceManifest == "" || packageParameters.Name == "" || packageParameters.Version == "" || packageParameters.Private || packageParameters.PackedName != packageParameters.Name || packageParameters.PackedVersion != packageParameters.Version {
		return npmValidationError(IDUnexpectedExternalParameters, "externalParameters.package", "package identity and packed identity must be complete and equal")
	}
	if strings.ContainsAny(packageParameters.TarballName, `/\\`) || !strings.HasSuffix(packageParameters.TarballName, ".tgz") {
		return npmValidationError(IDNPMSubjectMismatch, "externalParameters.package.tarball_name", "tarball name must be a safe tgz basename")
	}
	wantURL, err := npmRegistryPackageURL(parameters.Publish.ResolvedRegistryURL, packageParameters.Name, packageParameters.Version)
	if err != nil || packageParameters.PackageURL != wantURL {
		return npmValidationError("windlass.verify.error.package-url-mismatch", "externalParameters.package.package_url", "registry package URL is not canonical")
	}
	_, err = NPMPackagePURL(packageParameters.Name, packageParameters.Version)
	return err
}

func validatePackageManagerParameters(parameters PackageManagerParameters) error {
	if parameters.Name != ManagerNPM && parameters.Name != ManagerPNPM && parameters.Name != ManagerYarn || !minimumVersion(parameters.Version, 0, 0, 0) {
		return npmValidationError(IDUnexpectedExternalParameters, "externalParameters.package_manager", "package manager name and exact version are invalid")
	}
	if parameters.Root == "" || path.IsAbs(parameters.Root) || strings.Contains(parameters.Root, "\\") || strings.HasPrefix(path.Clean(parameters.Root), "..") {
		return npmValidationError(IDUnexpectedExternalParameters, "externalParameters.package_manager.root", "package-manager root must be repository relative")
	}
	if parameters.SelectionSource == SelectionLockfile {
		if parameters.SelectionManifest != nil || parameters.SelectionManifestPath != nil || parameters.SelectionLockfilePath == nil {
			return npmValidationError(IDUnexpectedExternalParameters, "externalParameters.package_manager", "lockfile selection paths have the wrong shape")
		}
	} else if (parameters.SelectionSource != SelectionPackageManager && parameters.SelectionSource != SelectionDevEngines) || parameters.SelectionManifest == nil || parameters.SelectionManifestPath == nil || parameters.SelectionLockfilePath != nil {
		return npmValidationError(IDUnexpectedExternalParameters, "externalParameters.package_manager", "manifest selection paths have the wrong shape")
	}
	if parameters.Name == ManagerYarn {
		if parameters.SelectionSource != SelectionPackageManager || !minimumVersion(parameters.Version, 4, 0, 0) || parameters.YarnInstallMode != "immutable" {
			return npmValidationError(IDUnexpectedExternalParameters, "externalParameters.package_manager", "Yarn requires packageManager selection, v4+, and immutable mode")
		}
	} else if parameters.YarnInstallMode != "" {
		return npmValidationError(IDUnexpectedExternalParameters, "externalParameters.package_manager.yarn_install_mode", "Yarn install mode is forbidden for npm and pnpm")
	}
	return nil
}

func validatePublishParameters(parameters PublishParameters) error {
	registry, registryErr := normalizeRegistryURL(parameters.ResolvedRegistryURL)
	if registryErr != nil || registry.String() != parameters.ResolvedRegistryURL || parameters.ResolvedDistTag == "" || !parameters.TrustedPublishing || !parameters.ProvenanceFile || parameters.PackageIdentityPreexisting == nil || !*parameters.PackageIdentityPreexisting || parameters.PackageVersionPreexisting == nil || *parameters.PackageVersionPreexisting {
		return npmValidationError(IDUnexpectedExternalParameters, "externalParameters.publish", "npmjs publish intent must be tokenless, provenance-file enabled, and target an absent version")
	}
	if parameters.EffectiveAccess != "existing-package-access" && parameters.EffectiveAccess != "public" && parameters.EffectiveAccess != "restricted" {
		return npmValidationError(IDUnexpectedExternalParameters, "externalParameters.publish.effective_access", "effective access is not enumerated")
	}
	if parameters.PublishAccessOption != nil && *parameters.PublishAccessOption != "public" && *parameters.PublishAccessOption != "restricted" {
		return npmValidationError(IDUnexpectedExternalParameters, "externalParameters.publish.publish_access_option", "publish access option is not enumerated")
	}
	wantCustomSupport := ""
	if parameters.ResolvedRegistryURL != "https://registry.npmjs.org/" {
		wantCustomSupport = "unsupported-but-not-blocked"
	}
	if parameters.CustomRegistrySupport != wantCustomSupport {
		return npmValidationError(IDUnexpectedExternalParameters, "externalParameters.publish.custom_registry_support", "custom registry marker is forbidden for npmjs")
	}
	return nil
}

func validateOptionalExternalFields(parameters ExternalParameters) error {
	allowedConsumerFields := map[string]bool{"exports": true, "main": true, "type": true, "bin": true, "types": true, "typings": true, "typesVersions": true, "files": true}
	for name := range parameters.Package.ConsumerSurface {
		if !allowedConsumerFields[name] {
			return npmValidationError(IDUnexpectedExternalParameters, "externalParameters.package.consumer_surface", "consumer surface contains an unknown field")
		}
	}
	for _, file := range parameters.Package.PackedFiles {
		if file == "" || path.IsAbs(file) || strings.Contains(file, "\\") || strings.HasPrefix(path.Clean(file), "..") {
			return npmValidationError(IDUnexpectedExternalParameters, "externalParameters.package.packed_files", "packed file path is unsafe")
		}
	}
	return nil
}

func validateResolvedDependencies(values []provenance.ResourceDescriptor, parameters ExternalParameters) error {
	byName := make(map[string][]provenance.ResourceDescriptor, len(values))
	for _, value := range values {
		switch value.Name {
		case "lockfile", "package-manager-distribution", "runner-image":
			byName[value.Name] = append(byName[value.Name], value)
		default:
			return npmValidationError(IDResolvedDependenciesUnexpectedEntry, "resolvedDependencies", "dependency name is not enumerated")
		}
	}
	if len(byName["lockfile"]) != 1 || len(byName["runner-image"]) != 1 {
		if len(byName["lockfile"]) != 1 {
			return npmValidationError(IDResolvedDependenciesLockfile, "resolvedDependencies.lockfile", "exactly one lockfile descriptor is required")
		}
		return npmValidationError(IDResolvedDependenciesRunnerImage, "resolvedDependencies.runner-image", "exactly one runner-image descriptor is required")
	}
	wantDistribution := parameters.PackageManager.Name != ManagerNPM
	if len(byName["package-manager-distribution"]) != boolCount(wantDistribution) {
		return npmValidationError(IDResolvedDependenciesDistribution, "resolvedDependencies.package-manager-distribution", "distribution descriptor cardinality must match the selected manager")
	}
	if err := validateLockfileDescriptor(byName["lockfile"][0], parameters); err != nil {
		return err
	}
	if wantDistribution {
		if err := validateDistributionDescriptor(byName["package-manager-distribution"][0], parameters); err != nil {
			return err
		}
	}
	return validateRunnerDescriptor(byName["runner-image"][0], parameters.Runtime.NodeVersion)
}

func validateLockfileDescriptor(value provenance.ResourceDescriptor, parameters ExternalParameters) error {
	lockfilePath := selectedLockfilePath(parameters.PackageManager)
	wantURI := "git+" + parameters.Source.Repository + "@" + parameters.Source.Revision + "#" + lockfilePath
	if value.URI != wantURI || len(value.Digest) != 1 || digestEncodingInvalid(value.Digest["sha256"], 64) || len(value.Annotations) != 6 {
		return npmValidationError(IDResolvedDependenciesLockfile, "resolvedDependencies.lockfile", "lockfile URI, digest, or annotation shape is invalid")
	}
	wantManifestPath := any(nil)
	if parameters.PackageManager.SelectionManifestPath != nil {
		wantManifestPath = *parameters.PackageManager.SelectionManifestPath
	}
	wants := map[string]any{
		"package_manager":              parameters.PackageManager.Name,
		"package_manager_root":         parameters.PackageManager.Root,
		"selection_source":             parameters.PackageManager.SelectionSource,
		"selection_manifest_path":      wantManifestPath,
		"selection_lockfile_path":      lockfilePath,
		"stale_non_selected_lockfiles": nonNilStrings(parameters.PackageManager.IgnoredLockfilePaths),
	}
	if !annotationsEqual(value.Annotations, wants) {
		return npmValidationError(IDResolvedDependenciesLockfile, "resolvedDependencies.lockfile.annotations", "lockfile annotations differ from external parameters")
	}
	return nil
}

func validateDistributionDescriptor(value provenance.ResourceDescriptor, parameters ExternalParameters) error {
	authority := "registry-integrity"
	if parameters.PackageManager.Name == ManagerYarn {
		authority = "download-hash"
	}
	if value.URI == "" || len(value.Digest) != 1 || digestEncodingInvalid(value.Digest["sha512"], 128) || len(value.Annotations) != 4 || !strings.HasPrefix(value.URI, "https://") {
		return npmValidationError(IDResolvedDependenciesDistribution, "resolvedDependencies.package-manager-distribution", "distribution URI, SHA-512, or annotation shape is invalid")
	}
	wants := map[string]any{"digest_authority": authority, "package_manager": parameters.PackageManager.Name, "package_manager_version": parameters.PackageManager.Version, "acquisition_source": "corepack"}
	if !annotationsEqual(value.Annotations, wants) {
		return npmValidationError(IDResolvedDependenciesDistribution, "resolvedDependencies.package-manager-distribution.annotations", "distribution annotations differ from the selected manager")
	}
	return nil
}

func validateRunnerDescriptor(value provenance.ResourceDescriptor, nodeVersion string) error {
	if value.URI == "" || len(value.Digest) != 0 || len(value.Annotations) != 3 {
		return npmValidationError(IDResolvedDependenciesRunnerImage, "resolvedDependencies.runner-image", "runner-image must have URI and annotations without a digest")
	}
	parsed, err := url.Parse(value.URI)
	if err != nil || parsed.Scheme != "https" || parsed.Host != "github.com" || !strings.HasPrefix(parsed.Path, "/actions/runner-images/") {
		return npmValidationError(IDResolvedDependenciesRunnerImage, "resolvedDependencies.runner-image.uri", "runner-image URI must be the captured actions/runner-images software report")
	}
	for _, name := range []string{"image_os", "image_version", "node_version"} {
		var text string
		if json.Unmarshal(value.Annotations[name], &text) != nil || text == "" {
			return npmValidationError(IDResolvedDependenciesRunnerImage, "resolvedDependencies.runner-image.annotations", "runner-image annotations must be non-empty strings")
		}
		if name == "node_version" && text != "v"+nodeVersion {
			return npmValidationError(IDResolvedDependenciesRunnerImage, "resolvedDependencies.runner-image.annotations.node_version", "runner Node.js version differs from external parameters")
		}
	}
	return nil
}

func selectedLockfilePath(parameters PackageManagerParameters) string {
	if parameters.SelectionLockfilePath != nil {
		return *parameters.SelectionLockfilePath
	}
	name := map[Manager]string{ManagerNPM: "package-lock.json", ManagerPNPM: "pnpm-lock.yaml", ManagerYarn: "yarn.lock"}[parameters.Name]
	if parameters.Root == "." {
		return name
	}
	return path.Join(parameters.Root, name)
}

func npmRegistryPackageURL(registry, name, version string) (string, error) {
	normalized, err := normalizeRegistryURL(registry)
	if err != nil || normalized.String() != registry || invalidPURLText(name) || invalidPURLText(version) {
		return "", fmt.Errorf("unsupported registry package URL input")
	}
	return registry + percentEncode(name) + "/" + percentEncode(version), nil
}

func percentEncode(value string) string {
	const hexadecimal = "0123456789ABCDEF"
	var builder strings.Builder
	for _, character := range []byte(value) {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || strings.ContainsRune("-._~", rune(character)) {
			builder.WriteByte(character)
			continue
		}
		builder.WriteByte('%')
		builder.WriteByte(hexadecimal[character>>4])
		builder.WriteByte(hexadecimal[character&0x0f])
	}
	return builder.String()
}

func minimumVersion(value string, minimumMajor, minimumMinor, minimumPatch int) bool {
	matches := exactSemverPattern.FindStringSubmatch(value)
	if len(matches) != 4 {
		return false
	}
	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return false
	}
	minor, err := strconv.Atoi(matches[2])
	if err != nil {
		return false
	}
	patch, err := strconv.Atoi(matches[3])
	if err != nil {
		return false
	}
	if major != minimumMajor {
		return major > minimumMajor
	}
	if minor != minimumMinor {
		return minor > minimumMinor
	}
	return patch >= minimumPatch
}

func digestEncodingInvalid(value string, length int) bool {
	if len(value) != length {
		return true
	}
	for _, character := range value {
		if character < '0' || character > '9' && character < 'a' || character > 'f' {
			return true
		}
	}
	return false
}

func annotationsEqual(got map[string]json.RawMessage, want map[string]any) bool {
	if len(got) != len(want) {
		return false
	}
	for name, wantValue := range want {
		encoded, err := json.Marshal(wantValue)
		if err != nil {
			return false
		}
		equal, err := canonicaljson.Equal(got[name], encoded)
		if err != nil || !equal {
			return false
		}
	}
	return true
}

func equalJSONValues(left, right any) (bool, error) {
	leftJSON, err := json.Marshal(left)
	if err != nil {
		return false, err
	}
	rightJSON, err := json.Marshal(right)
	if err != nil {
		return false, err
	}
	return canonicaljson.Equal(leftJSON, rightJSON)
}

func dependencyMismatch(name string) error {
	switch name {
	case "lockfile":
		return npmValidationError(IDResolvedDependenciesLockfile, "resolvedDependencies.lockfile", "lockfile differs from verified build metadata")
	case "package-manager-distribution":
		return npmValidationError(IDResolvedDependenciesDistribution, "resolvedDependencies.package-manager-distribution", "distribution differs from verified build metadata")
	case "runner-image":
		return npmValidationError(IDResolvedDependenciesRunnerImage, "resolvedDependencies.runner-image", "runner image differs from verified build metadata")
	default:
		return npmValidationError(IDResolvedDependenciesUnexpectedEntry, "resolvedDependencies", "dependency name is not enumerated")
	}
}

func boolCount(value bool) int {
	if value {
		return 1
	}
	return 0
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

type npmProvenanceValidationError struct {
	diagnostic diagnostic.Diagnostic
}

func (validationError *npmProvenanceValidationError) Error() string {
	return validationError.diagnostic.ID + ": " + validationError.diagnostic.Field + ": " + validationError.diagnostic.Message
}

func (validationError *npmProvenanceValidationError) DiagnosticID() string {
	return validationError.diagnostic.ID
}

func npmValidationError(id, field, message string) error {
	entry, err := diagnostic.New(id, field, message)
	if err != nil {
		return fmt.Errorf("construct npm provenance diagnostic %q: %w", id, err)
	}
	entry.Field = field
	return &npmProvenanceValidationError{diagnostic: entry}
}

var _ provenance.ProfileValidator = npmProfileValidator{}
