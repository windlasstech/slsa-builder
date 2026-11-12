package npmprofile

import (
	"encoding/json"
	"path"
	"strings"

	"github.com/windlasstech/slsa-builder/internal/digest"
	"github.com/windlasstech/slsa-builder/internal/identity"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

const (
	IDInvocationSource = "windlass.verify.error.resolved-dependencies-invocation-source"
	IDBuiltSource      = "windlass.verify.error.resolved-dependencies-built-source"
	IDPackageManifest  = "windlass.verify.error.resolved-dependencies-package-manifest"
)

func prependSourceDependencies(dependencies []provenance.ResourceDescriptor, selection Result, config WorkflowBuildMetadataConfig) ([]provenance.ResourceDescriptor, error) {
	invocationRevision := config.InvocationRevision
	if invocationRevision == "" && config.SourceRefInput == "" {
		invocationRevision = config.Revision
	}
	if identity.ValidateFullSHA(invocationRevision) != nil {
		return nil, npmValidationError(IDInvocationSource, "resolvedDependencies.invocation-source", "invocation commit must be a full lowercase SHA")
	}
	snapshot := selection.Package.SourceManifest
	sources := []provenance.ResourceDescriptor{
		{Name: "invocation-source", URI: "git+" + config.Repository + "@" + config.InvocationRef, Digest: map[string]string{"gitCommit": invocationRevision}},
		{Name: "built-source", URI: "git+" + config.Repository + "@" + config.Ref, Digest: map[string]string{"gitCommit": config.Revision}, Annotations: annotations(map[string]any{"ref_type": "tag"})},
		{Name: "package-manifest", URI: "git+" + config.Repository + "@" + config.Revision + "#" + escapeLockfileFragment(snapshot.Path), Digest: map[string]string{"sha256": digest.SumSHA256(snapshot.Content).String()}, MediaType: "application/json", Annotations: annotations(map[string]any{"workspace_root": workspaceRoot(selection.Package)})},
	}
	return append(sources, dependencies...), nil
}

func rawInput(value string) *string {
	if trimASCII(value) == "" {
		return nil
	}
	return optionalString(value)
}

func lockfileMediaType(manager Manager, filename string) string {
	switch {
	case manager == ManagerNPM && path.Base(filename) == "package-lock.json":
		return "application/json"
	case manager == ManagerPNPM && path.Base(filename) == "pnpm-lock.yaml", manager == ManagerYarn && path.Base(filename) == "yarn.lock":
		return "application/yaml"
	default:
		return ""
	}
}

func distributionMediaType(capture DistributionCapture) string {
	plain, _, failure := parseDescriptorVersion(capture.PackageManager, capture.PackageManagerVer)
	if failure != "" {
		return ""
	}
	if capture.AcquisitionSource == "corepack" && capture.DigestAuthority == "registry-integrity" && capture.PackageManager == ManagerPNPM && capture.URL == "https://registry.npmjs.org/pnpm/-/pnpm-"+plain+".tgz" {
		return "application/gzip"
	}
	if capture.AcquisitionSource == "corepack" && capture.DigestAuthority == "download-hash" && capture.PackageManager == ManagerYarn && capture.URL == "https://repo.yarnpkg.com/"+plain+"/packages/yarnpkg-cli/bin/yarn.js" {
		return "text/javascript"
	}
	return ""
}

func validateSourceDescriptor(value provenance.ResourceDescriptor, parameters ExternalParameters) error {
	id, ref := IDInvocationSource, parameters.Workflow.Ref
	if value.Name == "built-source" {
		id, ref = IDBuiltSource, effectiveBuiltRef(parameters)
	}
	if value.HasUnexpectedFields() || value.HasMediaType() || value.URI != "git+"+parameters.Workflow.Repository+"@"+ref || len(value.Digest) != 1 || identity.ValidateFullSHA(value.Digest["gitCommit"]) != nil {
		return npmValidationError(id, "resolvedDependencies."+value.Name, "source descriptor has invalid identity or shape")
	}
	if value.Name == "invocation-source" {
		if value.HasAnnotations() {
			return npmValidationError(id, "resolvedDependencies.invocation-source", "invocation annotations are forbidden")
		}
	} else if !strings.HasPrefix(ref, "refs/tags/") || !annotationsEqual(value.Annotations, map[string]any{"ref_type": "tag"}) {
		return npmValidationError(id, "resolvedDependencies.built-source", "built source requires the tag classifier")
	}
	return nil
}

func validateManifestDescriptor(value provenance.ResourceDescriptor, parameters ExternalParameters, commit string) error {
	wantURI := "git+" + parameters.Workflow.Repository + "@" + commit + "#" + escapeLockfileFragment(packageManifestPath(parameters.Package.Directory))
	if value.HasUnexpectedFields() || value.URI != wantURI || value.MediaType != "application/json" || len(value.Digest) != 1 || digestEncodingInvalid(value.Digest["sha256"], 64) || len(value.Annotations) != 1 {
		return npmValidationError(IDPackageManifest, "resolvedDependencies.package-manifest", "manifest descriptor has invalid identity or shape")
	}
	raw, present := value.Annotations["workspace_root"]
	if !present {
		return npmValidationError(IDPackageManifest, "resolvedDependencies.package-manifest", "workspace root annotation is required")
	}
	if string(raw) != "null" {
		var root string
		if json.Unmarshal(raw, &root) != nil || !safeDependencyPath(root) || root != "." && !strings.HasPrefix(parameters.Package.Directory, root+"/") && root != parameters.Package.Directory {
			return npmValidationError(IDPackageManifest, "resolvedDependencies.package-manifest", "workspace root must contain the selected package")
		}
	}
	return nil
}

func safeDependencyPath(value string) bool {
	if value == "" || path.IsAbs(value) || path.Clean(value) != value || strings.ContainsAny(value, "\\\x00") {
		return false
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == ".." {
			return false
		}
	}
	return true
}
