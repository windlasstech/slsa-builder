package npmprofile

import (
	"encoding/json"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
	"github.com/windlasstech/slsa-builder/internal/diagnostic"
	"github.com/windlasstech/slsa-builder/internal/provenance"
	"golang.org/x/mod/semver"
)

type npmProfileValidator struct {
	parameters        ExternalParameters
	encodedParameters json.RawMessage
	dependencies      []provenance.ResourceDescriptor
	sha256            string
	sha512            string
	packageName       string
	packageVersion    string
}

func (validator npmProfileValidator) ValidateSubject(subject provenance.Subject) error {
	want, err := NPMPackagePURL(validator.packageName, validator.packageVersion)
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
	if err := validateFinalExternalParameters(parameters, validator.parameters.Workflow.Repository); err != nil {
		return err
	}
	equal, err := canonicaljson.Equal(encoded, validator.encodedParameters)
	if err != nil || !equal {
		return npmValidationError(IDUnexpectedExternalParameters, "predicate.buildDefinition.externalParameters", "signed external parameters differ from the verified build metadata")
	}
	return nil
}

func (validator npmProfileValidator) ValidateResolvedDependencies(values []provenance.ResourceDescriptor) error {
	if err := validateFinalDependencies(values, validator.parameters); err != nil {
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

func validFullGitRef(ref string) bool {
	if !strings.HasPrefix(ref, "refs/") || strings.HasSuffix(ref, "/") {
		return false
	}
	components := strings.Split(ref, "/")
	if len(components) < 3 {
		return false
	}
	for _, component := range components[1:] {
		if component == "" || strings.HasPrefix(component, ".") || strings.HasSuffix(component, ".") || strings.HasSuffix(component, ".lock") {
			return false
		}
	}
	return validTagName(strings.TrimPrefix(ref, "refs/"))
}

func validCallerWorkflowPath(value string) bool {
	const prefix = ".github/workflows/"
	if !strings.HasPrefix(value, prefix) || strings.ContainsAny(value, "%\\\x00") {
		return false
	}
	filename := strings.TrimPrefix(value, prefix)
	if filename == "" || strings.Contains(filename, "/") || strings.HasPrefix(filename, ".") {
		return false
	}
	return strings.HasSuffix(filename, ".yml") || strings.HasSuffix(filename, ".yaml")
}

func validatePackageManagerParameters(parameters PackageManagerParameters) error {
	if parameters.Name != ManagerNPM && parameters.Name != ManagerPNPM && parameters.Name != ManagerYarn {
		return npmValidationError(IDUnexpectedExternalParameters, "externalParameters.package_manager", "package manager name and exact version are invalid")
	}
	// ADRs 0092-0094: the recorded version may carry one closed
	// +<algorithm>.<hex> integrity digest suffix (never on npm, ADR 0095).
	// Exact-version and version-bound rules below apply to the plain version
	// with the suffix stripped.
	plain, _, failureID := parseDescriptorVersion(parameters.Name, parameters.Version)
	if failureID != "" {
		return npmValidationError(IDPackageManagerDigestMalformed, "externalParameters.package_manager.version", "package manager version digest suffix violates the closed +<algorithm>.<hex> grammar")
	}
	if !minimumVersion(plain, 0, 0, 0) {
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
		// ADR 0091: Yarn selection may come from packageManager or
		// devEngines.packageManager; lockfile inference stays invalid.
		manifestSelected := parameters.SelectionSource == SelectionPackageManager || parameters.SelectionSource == SelectionDevEngines
		if !manifestSelected || !minimumVersion(plain, 4, 0, 0) || !majorVersionBelow(plain, 6) || parameters.YarnInstallMode != "immutable" {
			return npmValidationError(IDUnexpectedExternalParameters, "externalParameters.package_manager", "Yarn requires packageManager or devEngines selection, an exact v4 or v5 version, and immutable mode")
		}
	} else if parameters.YarnInstallMode != "" {
		return npmValidationError(IDUnexpectedExternalParameters, "externalParameters.package_manager.yarn_install_mode", "Yarn install mode is forbidden for npm and pnpm")
	}
	return nil
}

func validateNodeRuntimeDescriptor(value provenance.ResourceDescriptor) error {
	if value.HasUnexpectedFields() {
		return npmValidationError(IDResolvedDependenciesNodeRuntime, "resolvedDependencies.node-runtime", "runtime descriptor contains unrecognized members")
	}
	want, err := nodeRuntimeDescriptor()
	if err != nil {
		return err
	}
	equal, err := equalJSONValues(value, want)
	if err != nil || !equal {
		return npmValidationError(IDResolvedDependenciesNodeRuntime, "resolvedDependencies.node-runtime", "runtime descriptor must match the embedded linux-x64 pin and closed annotations")
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

// minimumVersion and majorVersionBelow compare full SemVer versions,
// including prerelease descriptors such as yarn@5.0.0-rc.1 that selection
// accepts: provenance validation must apply the same bounds or the
// fail-closed validator would reject a build the profile selected.
func minimumVersion(value string, minimumMajor, minimumMinor, minimumPatch int) bool {
	if !exactSemver(value) {
		return false
	}
	minimum := "v" + strconv.Itoa(minimumMajor) + "." + strconv.Itoa(minimumMinor) + "." + strconv.Itoa(minimumPatch)
	return semver.Compare("v"+value, minimum) >= 0
}

func majorVersionBelow(value string, maximumMajor int) bool {
	if !exactSemver(value) {
		return false
	}
	major, err := strconv.Atoi(strings.TrimPrefix(semver.Major("v"+value), "v"))
	return err == nil && major < maximumMajor
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
	case "invocation-source":
		return npmValidationError(IDInvocationSource, "resolvedDependencies.invocation-source", "invocation source differs from verified build metadata")
	case "built-source":
		return npmValidationError(IDBuiltSource, "resolvedDependencies.built-source", "built source differs from verified build metadata")
	case "package-manifest":
		return npmValidationError(IDPackageManifest, "resolvedDependencies.package-manifest", "manifest descriptor differs from verified build metadata")
	case "lockfile":
		return npmValidationError(IDResolvedDependenciesLockfile, "resolvedDependencies.lockfile", "lockfile differs from verified build metadata")
	case "node-runtime":
		return npmValidationError(IDResolvedDependenciesNodeRuntime, "resolvedDependencies.node-runtime", "runtime differs from verified build metadata")
	case "package-manager-distribution":
		return npmValidationError(IDResolvedDependenciesDistribution, "resolvedDependencies.package-manager-distribution", "distribution differs from verified build metadata")
	case "runner-image":
		return npmValidationError(IDResolvedDependenciesRunnerImage, "resolvedDependencies.runner-image", "runner image differs from verified build metadata")
	default:
		return npmValidationError(IDResolvedDependenciesUnexpectedEntry, "resolvedDependencies", "dependency name is not enumerated")
	}
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

// DiagnosticCheck exposes the diagnostic's check path so adapters can emit it
// without hardcoding a per-call-site constant.
func (validationError *npmProvenanceValidationError) DiagnosticCheck() string {
	return validationError.diagnostic.Check
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
