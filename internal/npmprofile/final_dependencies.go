package npmprofile

import (
	"encoding/json"
	"path"
	"strings"

	"github.com/windlasstech/slsa-builder/internal/provenance"
)

func orderedProducerDependencies(values []provenance.ResourceDescriptor) []provenance.ResourceDescriptor {
	order := []string{"invocation-source", "built-source", "package-manifest", "lockfile", "node-runtime", "package-manager-distribution", "runner-image"}
	ordered := make([]provenance.ResourceDescriptor, 0, len(values))
	for _, name := range order {
		for _, descriptor := range values {
			if descriptor.Name == name {
				ordered = append(ordered, descriptor)
			}
		}
	}
	return cloneNPMDependencies(ordered)
}

func finalDependencySet(values []provenance.ResourceDescriptor) (map[string]provenance.ResourceDescriptor, error) {
	ids := map[string]string{"invocation-source": IDInvocationSource, "built-source": IDBuiltSource, "package-manifest": IDPackageManifest, "lockfile": IDResolvedDependenciesLockfile, "node-runtime": IDResolvedDependenciesNodeRuntime, "package-manager-distribution": IDResolvedDependenciesDistribution, "runner-image": IDResolvedDependenciesRunnerImage}
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		id, known := ids[value.Name]
		if !known {
			return nil, npmValidationError(IDResolvedDependenciesUnexpectedEntry, "resolvedDependencies", "dependency name is not enumerated")
		}
		if seen[value.Name] || value.HasUnexpectedFields() {
			return nil, npmValidationError(id, "resolvedDependencies."+value.Name, "dependency is duplicated or contains unknown members")
		}
		seen[value.Name] = true
	}
	for _, name := range []string{"invocation-source", "built-source", "package-manifest", "lockfile", "node-runtime", "runner-image"} {
		if !seen[name] {
			return nil, npmValidationError(ids[name], "resolvedDependencies."+name, "required dependency is absent")
		}
	}
	byName := make(map[string]provenance.ResourceDescriptor, len(values))
	for _, value := range values {
		byName[value.Name] = value
	}
	return byName, nil
}

func validateFinalDependencies(values []provenance.ResourceDescriptor, parameters ExternalParameters) error {
	byName, err := finalDependencySet(values)
	if err != nil {
		return err
	}
	for _, name := range []string{"invocation-source", "built-source"} {
		if err := validateSourceDescriptor(byName[name], parameters); err != nil {
			return err
		}
	}
	built := byName["built-source"]
	if parameters.Source.Ref == "" && byName["invocation-source"].Digest["gitCommit"] != built.Digest["gitCommit"] {
		return npmValidationError(IDBuiltSource, "resolvedDependencies.built-source", "omitted selector requires identical source identities")
	}
	if err := validateManifestDescriptor(byName["package-manifest"], parameters, built.Digest["gitCommit"]); err != nil {
		return err
	}
	manager, err := finalLockfileManager(byName["lockfile"], parameters, built.Digest["gitCommit"])
	if err != nil {
		return err
	}
	var managerRoot string
	if err := json.Unmarshal(byName["lockfile"].Annotations["package_manager_root"], &managerRoot); err != nil {
		return err
	}
	var workspace *string
	if managerRoot != parameters.Package.Directory {
		workspace = &managerRoot
	}
	if !annotationsEqual(byName["package-manifest"].Annotations, map[string]any{"workspace_root": workspace}) {
		return npmValidationError(IDPackageManifest, "resolvedDependencies.package-manifest", "manifest workspace classification differs from the selected manager root")
	}
	distribution, present := byName["package-manager-distribution"]
	if present != (manager != ManagerNPM) {
		return npmValidationError(IDResolvedDependenciesDistribution, "resolvedDependencies.package-manager-distribution", "distribution cardinality must match the selected lockfile manager")
	}
	if present {
		if err := validateFinalDistribution(distribution, manager); err != nil {
			return err
		}
	}
	if err := validateNodeRuntimeDescriptor(byName["node-runtime"]); err != nil {
		return err
	}
	return validateRunnerDescriptor(byName["runner-image"])
}

func finalLockfileManager(value provenance.ResourceDescriptor, parameters ExternalParameters, commit string) (Manager, error) {
	invalid := func() (Manager, error) {
		return "", npmValidationError(IDResolvedDependenciesLockfile, "resolvedDependencies.lockfile", "lockfile identity, representation, or selection annotations are invalid")
	}
	var annotation struct {
		Manager  Manager         `json:"package_manager"`
		Root     string          `json:"package_manager_root"`
		Source   SelectionSource `json:"selection_source"`
		Manifest *string         `json:"selection_manifest_path"`
		Lockfile string          `json:"selection_lockfile_path"`
		Stale    []string        `json:"stale_non_selected_lockfiles"`
	}
	keys := []string{"package_manager", "package_manager_root", "selection_source", "selection_manifest_path", "selection_lockfile_path", "stale_non_selected_lockfiles"}
	if !exactObjectKeys(value.Annotations, keys, nil) {
		return invalid()
	}
	raw, err := json.Marshal(value.Annotations)
	if err != nil || json.Unmarshal(raw, &annotation) != nil {
		return invalid()
	}
	if !safeDependencyPath(annotation.Root) || !safeDependencyPath(annotation.Lockfile) || path.Dir(annotation.Lockfile) != annotation.Root || annotation.Stale == nil {
		return invalid()
	}
	mediaType := lockfileMediaType(annotation.Manager, annotation.Lockfile)
	if mediaType == "" || value.MediaType != mediaType || len(value.Digest) != 1 || digestEncodingInvalid(value.Digest["sha256"], 64) || value.URI != "git+"+parameters.Workflow.Repository+"@"+commit+"#"+escapeLockfileFragment(annotation.Lockfile) {
		return invalid()
	}
	switch annotation.Source {
	case SelectionLockfile:
		if annotation.Manager != ManagerNPM || annotation.Manifest != nil {
			return invalid()
		}
	case SelectionPackageManager, SelectionDevEngines:
		if annotation.Manifest == nil || !safeDependencyPath(*annotation.Manifest) || *annotation.Manifest != packageManifestPath(parameters.Package.Directory) && *annotation.Manifest != packageManifestPath(annotation.Root) {
			return invalid()
		}
	default:
		return invalid()
	}
	seen := map[string]bool{}
	for _, stale := range annotation.Stale {
		if seen[stale] || !safeDependencyPath(stale) || path.Dir(stale) != annotation.Root || stale == annotation.Lockfile {
			return invalid()
		}
		switch path.Base(stale) {
		case "package-lock.json", "pnpm-lock.yaml", "yarn.lock":
		default:
			return invalid()
		}
		seen[stale] = true
	}
	if annotation.Root != "." && parameters.Package.Directory != annotation.Root && !strings.HasPrefix(parameters.Package.Directory, annotation.Root+"/") {
		return invalid()
	}
	return annotation.Manager, nil
}

func validateFinalDistribution(value provenance.ResourceDescriptor, manager Manager) error {
	invalid := func() error {
		return npmValidationError(IDResolvedDependenciesDistribution, "resolvedDependencies.package-manager-distribution", "distribution representation or captured selection is invalid")
	}
	var capture DistributionCapture
	var version string
	if !exactObjectKeys(value.Annotations, []string{"digest_authority", "package_manager", "package_manager_version", "acquisition_source"}, nil) {
		return invalid()
	}
	if json.Unmarshal(value.Annotations["package_manager"], &capture.PackageManager) != nil || json.Unmarshal(value.Annotations["package_manager_version"], &version) != nil || json.Unmarshal(value.Annotations["digest_authority"], &capture.DigestAuthority) != nil || json.Unmarshal(value.Annotations["acquisition_source"], &capture.AcquisitionSource) != nil {
		return invalid()
	}
	capture.PackageManagerVer, capture.URL = version, value.URI
	plain, declared, failure := parseDescriptorVersion(manager, version)
	if failure != "" || capture.PackageManager != manager || !minimumVersion(plain, 0, 0, 0) {
		return invalid()
	}
	if manager == ManagerPNPM && (!minimumVersion(plain, 10, 0, 0) || !majorVersionBelow(plain, 12)) || manager == ManagerYarn && (!minimumVersion(plain, 4, 0, 0) || !majorVersionBelow(plain, 6)) {
		return invalid()
	}
	media := distributionMediaType(capture)
	if media == "" || value.MediaType != media || len(value.Digest) != 1 || digestEncodingInvalid(value.Digest["sha512"], 128) {
		return invalid()
	}
	if declared != nil && declared.Algorithm == "sha512" && declared.Hex != value.Digest["sha512"] {
		return invalid()
	}
	return nil
}
