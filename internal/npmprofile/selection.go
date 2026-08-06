package npmprofile

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/windlasstech/slsa-builder/internal/diagnostic"
	"golang.org/x/mod/semver"
)

var lockfileByManager = map[Manager]string{
	ManagerNPM:  "package-lock.json",
	ManagerPNPM: "pnpm-lock.yaml",
	ManagerYarn: "yarn.lock",
}

// Analyze resolves one package and produces a closed diagnostic report without executing a package manager.
func Analyze(config Config) (Result, error) {
	resolved, metadata, failureID, canonicalRepository := resolvePackage(config)
	if failureID != "" {
		return rejectedResult(failureID, metadata)
	}

	result := Result{Package: Package{
		Directory:                    resolved.directory,
		RealDirectory:                resolved.directoryReal,
		RealManagerRoot:              resolved.managerRootReal,
		ManagerRoot:                  resolved.managerRoot,
		ManagerRootRelativeDirectory: relativePackageDirectory(resolved),
		Name:                         resolved.manifest.Name,
		Version:                      resolved.manifest.Version,
		Repository:                   canonicalRepository,
	}}

	selection, diagnostics, failureID := selectManager(resolved)
	result.Manager = selection
	if failureID != "" {
		rejected, err := rejectedResult(failureID, metadata)
		rejected.Package = result.Package
		rejected.Manager = result.Manager
		return rejected, err
	}
	report, err := diagnostic.Build(nil, diagnostics, metadata)
	if err != nil {
		return Result{}, fmt.Errorf("build npm profile report: %w", err)
	}
	result.Report = report
	return result, nil
}

func selectManager(resolved resolvedPackage) (ManagerSelection, []diagnostic.Diagnostic, string) {
	candidates, failureID := manifestCandidates(resolved)
	if failureID != "" {
		return ManagerSelection{}, nil, failureID
	}
	if len(candidates) > 0 {
		selected := candidates[0]
		for _, candidate := range candidates[1:] {
			if candidate.name != selected.name ||
				(selected.name != ManagerNPM && candidate.version != selected.version) {
				return ManagerSelection{}, nil, IDPackageManagerConflict
			}
		}
		selection := ManagerSelection{
			Name:                  selected.name,
			Version:               effectiveVersion(selected),
			Source:                selected.source,
			SelectionManifestPath: selected.manifestPath,
		}
		return validateLockfiles(resolved, selection, true)
	}
	return inferFromLockfile(resolved)
}

func manifestCandidates(resolved resolvedPackage) ([]managerCandidate, string) {
	sources := []struct {
		manifest manifest
		path     string
	}{
		{manifest: resolved.manifest, path: resolved.manifestPath},
	}
	if resolved.rootManifest != nil {
		sources = append(sources, struct {
			manifest manifest
			path     string
		}{manifest: *resolved.rootManifest, path: resolved.rootManifestPath})
	}

	candidates := make([]managerCandidate, 0, len(sources)*2)
	for _, source := range sources {
		if len(source.manifest.PackageManager) != 0 {
			candidate, failureID := parsePackageManager(source.manifest.PackageManager, source.path)
			if failureID != "" {
				return nil, failureID
			}
			candidates = append(candidates, candidate)
		}
		candidate, present, failureID := parseDevEngines(source.manifest.DevEngines, source.path)
		if failureID != "" {
			return nil, failureID
		}
		if present {
			candidates = append(candidates, candidate)
		}
	}
	return candidates, ""
}

func parsePackageManager(raw jsonRaw, manifestPath string) (managerCandidate, string) {
	var descriptor string
	if json.Unmarshal(raw, &descriptor) != nil {
		return managerCandidate{}, IDPackageManagerConflict
	}
	name, version, found := strings.Cut(descriptor, "@")
	if !found || name == "" || version == "" || strings.Contains(version, "+") {
		if name == string(ManagerYarn) {
			return managerCandidate{}, IDYarnSelectionInvalid
		}
		if name == string(ManagerPNPM) {
			return managerCandidate{}, IDPackageManagerVersionRequired
		}
		return managerCandidate{}, IDPackageManagerConflict
	}
	candidate := managerCandidate{name: Manager(name), version: version, source: SelectionPackageManager, manifestPath: manifestPath}
	switch candidate.name {
	case ManagerNPM:
		return candidate, ""
	case ManagerPNPM:
		if !exactSemver(version) {
			return managerCandidate{}, IDPackageManagerVersionRequired
		}
		return candidate, ""
	case ManagerYarn:
		if !exactSemver(version) || semver.Compare("v"+version, "v4.0.0") < 0 {
			return managerCandidate{}, IDYarnSelectionInvalid
		}
		return candidate, ""
	default:
		return managerCandidate{}, IDPackageManagerConflict
	}
}

func parseDevEngines(raw jsonRaw, manifestPath string) (managerCandidate, bool, string) {
	if len(raw) == 0 {
		return managerCandidate{}, false, ""
	}
	var devEngines map[string]json.RawMessage
	if json.Unmarshal(raw, &devEngines) != nil {
		return managerCandidate{}, false, IDPackageManagerConflict
	}
	packageManagerRaw, present := devEngines["packageManager"]
	if !present {
		return managerCandidate{}, false, ""
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(packageManagerRaw, &object) != nil {
		return managerCandidate{}, false, IDPackageManagerConflict
	}
	for key := range object {
		if key != "name" && key != "version" && key != "onFail" {
			return managerCandidate{}, false, IDPackageManagerConflict
		}
	}
	var name string
	if json.Unmarshal(object["name"], &name) != nil || name == "" {
		return managerCandidate{}, false, IDPackageManagerConflict
	}
	var version string
	if rawVersion, ok := object["version"]; ok && json.Unmarshal(rawVersion, &version) != nil {
		return managerCandidate{}, false, IDPackageManagerConflict
	}
	if rawOnFail, ok := object["onFail"]; ok {
		var onFail string
		if json.Unmarshal(rawOnFail, &onFail) != nil ||
			(onFail != "ignore" && onFail != "warn" && onFail != "error" && onFail != "download") {
			return managerCandidate{}, false, IDPackageManagerConflict
		}
	}
	candidate := managerCandidate{name: Manager(name), version: version, source: SelectionDevEngines, manifestPath: manifestPath}
	switch candidate.name {
	case ManagerNPM:
		return candidate, true, ""
	case ManagerPNPM:
		if !exactSemver(version) || strings.Contains(version, "+") {
			return managerCandidate{}, false, IDPackageManagerVersionRequired
		}
		return candidate, true, ""
	case ManagerYarn:
		return managerCandidate{}, false, IDYarnSelectionInvalid
	default:
		return managerCandidate{}, false, IDPackageManagerConflict
	}
}

func inferFromLockfile(resolved resolvedPackage) (ManagerSelection, []diagnostic.Diagnostic, string) {
	lockfiles, err := presentLockfiles(resolved.managerRootReal)
	if err != nil || len(lockfiles) == 0 || len(lockfiles) > 1 {
		return ManagerSelection{}, nil, IDPackageManagerConflict
	}
	manager := managerForLockfile(lockfiles[0])
	selection := ManagerSelection{
		Name:                  manager,
		Source:                SelectionLockfile,
		SelectionLockfilePath: joinRelative(resolved.managerRoot, lockfiles[0]),
		SelectedLockfilePath:  joinRelative(resolved.managerRoot, lockfiles[0]),
	}
	switch manager {
	case ManagerNPM:
		return selection, nil, ""
	case ManagerPNPM:
		return selection, nil, IDPackageManagerVersionRequired
	case ManagerYarn:
		return selection, nil, IDYarnSelectionInvalid
	default:
		return ManagerSelection{}, nil, IDPackageManagerConflict
	}
}

func validateLockfiles(resolved resolvedPackage, selection ManagerSelection, manifestSelected bool) (ManagerSelection, []diagnostic.Diagnostic, string) {
	lockfiles, err := presentLockfiles(resolved.managerRootReal)
	if err != nil {
		return selection, nil, IDRequiredLockfileMissing
	}
	required := lockfileByManager[selection.Name]
	present := false
	for _, lockfile := range lockfiles {
		if lockfile == required {
			present = true
			break
		}
	}
	if !present {
		return selection, nil, IDRequiredLockfileMissing
	}
	selection.SelectedLockfilePath = joinRelative(resolved.managerRoot, required)
	if !manifestSelected {
		return selection, nil, ""
	}
	warnings := make([]diagnostic.Diagnostic, 0, len(lockfiles)-1)
	for _, lockfile := range lockfiles {
		if lockfile == required {
			continue
		}
		ignoredPath := joinRelative(resolved.managerRoot, lockfile)
		selection.IgnoredLockfilePaths = append(selection.IgnoredLockfilePaths, ignoredPath)
		warning, newErr := diagnostic.New(diagnostic.IDStaleNonSelectedLockfile, "package_manager.lockfiles", "A non-selected supported lockfile is ignored as stale.")
		if newErr != nil {
			return selection, nil, IDPackageManagerConflict
		}
		warning.Field = "externalParameters.package_manager.ignored_lockfile_paths"
		warning.Actual = diagnostic.JSONValue(ignoredPath)
		warnings = append(warnings, warning)
	}
	sort.Strings(selection.IgnoredLockfilePaths)
	return selection, warnings, ""
}

func presentLockfiles(root string) ([]string, error) {
	lockfiles := make([]string, 0, len(lockfileByManager))
	for _, manager := range []Manager{ManagerNPM, ManagerPNPM, ManagerYarn} {
		name := lockfileByManager[manager]
		info, err := os.Lstat(filepath.Join(root, name))
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
				return nil, errors.New("lockfile is not regular")
			}
			lockfiles = append(lockfiles, name)
			continue
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
	}
	return lockfiles, nil
}

func rejectedResult(id string, metadata *diagnostic.DiagnosticMetadata) (Result, error) {
	entry, err := diagnostic.New(id, "npm_profile.selection", rejectionMessage(id))
	if err != nil {
		return Result{}, fmt.Errorf("construct npm profile diagnostic: %w", err)
	}
	report, err := diagnostic.Build(nil, []diagnostic.Diagnostic{entry}, metadata)
	if err != nil {
		return Result{}, fmt.Errorf("build npm profile rejection report: %w", err)
	}
	return Result{Report: report}, nil
}

func rejectionMessage(id string) string {
	switch id {
	case IDPackageManifestInvalid:
		return "The selected package manifest is missing or invalid."
	case IDPackageMetadataRequired:
		return "The selected package manifest requires name and version metadata."
	case IDPackagePrivate:
		return "The selected package is private and cannot be published by this profile."
	case IDPackageResolutionInvalid:
		return "The package directory does not resolve to exactly one package inside the repository."
	case IDPackageManagerConflict:
		return "Package-manager selection is unsupported, ambiguous, or conflicting."
	case IDPackageManagerVersionRequired:
		return "An exact pnpm package-manager version is required from manifest metadata."
	case IDYarnSelectionInvalid:
		return "Yarn requires top-level packageManager metadata selecting an exact Yarn v4 or newer version."
	case IDRequiredLockfileMissing:
		return "The selected package manager's required lockfile is missing."
	case IDPackageRepositoryIdentityMismatch:
		return "The package repository identity is missing, invalid, or does not match the caller repository."
	default:
		return "The npm profile selection policy rejected the package."
	}
}

func effectiveVersion(candidate managerCandidate) string {
	if candidate.name == ManagerNPM {
		return ""
	}
	return candidate.version
}

func exactSemver(version string) bool {
	return version != "" && semver.IsValid("v"+version)
}

func managerForLockfile(lockfile string) Manager {
	for manager, name := range lockfileByManager {
		if name == lockfile {
			return manager
		}
	}
	return ""
}

func relativePackageDirectory(resolved resolvedPackage) string {
	relative, err := filepath.Rel(resolved.managerRootReal, resolved.directoryReal)
	if err != nil {
		return ""
	}
	return canonicalRelative(relative)
}
