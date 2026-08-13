package npmprofile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFinalizeWorkflowBuildMetadataSourceRefDispatchRetry(t *testing.T) {
	repositoryRoot := t.TempDir()
	manifest := []byte(`{"name":"@windlass/slsa-builder","version":"1.2.3","repository":"https://github.com/example/project"}`)
	if err := os.WriteFile(filepath.Join(repositoryRoot, "package.json"), manifest, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repositoryRoot, "package-lock.json"), []byte(`{"lockfileVersion":3}`), 0o600); err != nil {
		t.Fatal(err)
	}
	selection := Result{
		Package: Package{Directory: ".", RealDirectory: repositoryRoot, RealManagerRoot: repositoryRoot, ManagerRoot: ".", Name: "@windlass/slsa-builder", Version: "1.2.3", Repository: "https://github.com/example/project"},
		Manager: ManagerSelection{Name: ManagerNPM, Version: "11.5.1", Source: SelectionPackageManager, SelectionManifestPath: "package.json", SelectedLockfilePath: "package-lock.json"},
	}
	build := BuildPackResult{
		PackageName: "@windlass/slsa-builder", PackageVersion: "1.2.3",
		TarballPath: filepath.Join(repositoryRoot, "windlass-slsa-builder-1.2.3.tgz"),
		SHA256:      mustSHA256(t, testSHA256), SHA512: mustSHA512(t, testSHA512),
		Packed:      PackedMetadata{Name: "@windlass/slsa-builder", Version: "1.2.3", Files: []string{"package.json"}},
		BuildScript: BuildScriptCapture{Present: true, Result: BuildScriptExecuted},
		Toolchain:   ToolchainCapture{NodeVersion: "v24.0.0", NPMVersion: "11.5.1", PackageManagerVersion: "11.5.1", Runner: RunnerCapture{ImageOS: "ubuntu24", ImageVersion: "20260801.1.0", IncludedSoftwareURL: "https://github.com/actions/runner-images/blob/main/images/ubuntu/Ubuntu2404-Readme.md"}},
	}
	metadata, err := FinalizeWorkflowBuildMetadata(selection, build, WorkflowBuildMetadataConfig{
		ArtifactName: "js-ts-npm-package-tarball-123456789-1", RegistryURLInput: "https://registry.npmjs.org/",
		EventName: "workflow_dispatch", RefType: "branch", Ref: "refs/tags/v1.2.3", Revision: testSourceSHA,
		SourceRefInput: "refs/tags/v1.2.3", InvocationRef: "refs/heads/main", InvocationRevision: testAttestSHA,
		WorkflowSHA: testSourceSHA, CallerWorkflowFilename: "release.yml",
		RegistryState: RegistryPreflightState{PackageExists: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	parameters, err := DecodeExternalParameters(metadata.ExternalParameters)
	if err != nil {
		t.Fatal(err)
	}
	if parameters.Source.Ref != "refs/tags/v1.2.3" || parameters.Source.Revision != testSourceSHA || parameters.Source.RefType != "tag" {
		t.Fatalf("built source = %#v", parameters.Source)
	}
	if parameters.Source.InputRef == nil || *parameters.Source.InputRef != "refs/tags/v1.2.3" ||
		parameters.Source.InvocationRef == nil || *parameters.Source.InvocationRef != "refs/heads/main" ||
		parameters.Source.InvocationRevision == nil || *parameters.Source.InvocationRevision != testAttestSHA {
		t.Fatalf("invocation record = %#v", parameters.Source)
	}
	if parameters.Release.Ref != "refs/tags/v1.2.3" || parameters.Release.VersionTag != "v1.2.3" {
		t.Fatalf("release = %#v", parameters.Release)
	}
	if got := metadata.ResolvedDependencies[0].URI; got != "git+https://github.com/example/project@"+testSourceSHA+"#package-lock.json" {
		t.Fatalf("lockfile URI = %q", got)
	}
}

func TestFinalizeWorkflowBuildMetadataSourceRefRequiresInvocationContext(t *testing.T) {
	selection := Result{Package: Package{Name: "@windlass/slsa-builder", Version: "1.2.3"}}
	config := WorkflowBuildMetadataConfig{
		SourceRefInput: "refs/tags/v1.2.3",
		Ref:            "refs/tags/v1.2.3",
		Revision:       testSourceSHA,
	}

	_, err := FinalizeWorkflowBuildMetadata(selection, BuildPackResult{}, config)
	requireNPMDiagnostic(t, err, IDUnexpectedExternalParameters)
}

func TestFinalizeWorkflowBuildMetadataRejectsWhitespaceSourceRef(t *testing.T) {
	repositoryRoot := t.TempDir()
	manifest := []byte(`{"name":"@windlass/slsa-builder","version":"1.2.3","repository":"https://github.com/example/project"}`)
	if err := os.WriteFile(filepath.Join(repositoryRoot, "package.json"), manifest, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repositoryRoot, "package-lock.json"), []byte(`{"lockfileVersion":3}`), 0o600); err != nil {
		t.Fatal(err)
	}
	selection := Result{
		Package: Package{Directory: ".", RealDirectory: repositoryRoot, RealManagerRoot: repositoryRoot, ManagerRoot: ".", Name: "@windlass/slsa-builder", Version: "1.2.3", Repository: "https://github.com/example/project"},
		Manager: ManagerSelection{Name: ManagerNPM, Version: "11.5.1", Source: SelectionPackageManager, SelectionManifestPath: "package.json", SelectedLockfilePath: "package-lock.json"},
	}
	build := BuildPackResult{
		PackageName: "@windlass/slsa-builder", PackageVersion: "1.2.3",
		TarballPath: filepath.Join(repositoryRoot, "windlass-slsa-builder-1.2.3.tgz"),
		SHA256:      mustSHA256(t, testSHA256), SHA512: mustSHA512(t, testSHA512),
		Packed:      PackedMetadata{Name: "@windlass/slsa-builder", Version: "1.2.3", Files: []string{"package.json"}},
		BuildScript: BuildScriptCapture{Present: true, Result: BuildScriptExecuted},
		Toolchain:   ToolchainCapture{NodeVersion: "v24.0.0", NPMVersion: "11.5.1", PackageManagerVersion: "11.5.1", Runner: RunnerCapture{ImageOS: "ubuntu24", ImageVersion: "20260801.1.0", IncludedSoftwareURL: "https://github.com/actions/runner-images/blob/main/images/ubuntu/Ubuntu2404-Readme.md"}},
	}
	config := WorkflowBuildMetadataConfig{
		ArtifactName: "js-ts-npm-package-tarball-123456789-1", RegistryURLInput: "https://registry.npmjs.org/",
		EventName: "push", RefType: "tag", Ref: "refs/tags/v1.2.3", Revision: testSourceSHA,
		WorkflowSHA: testSourceSHA, CallerWorkflowFilename: "release.yml",
		RegistryState: RegistryPreflightState{PackageExists: true},
	}
	config.SourceRefInput = " \t\n\r\v\f"
	_, err := FinalizeWorkflowBuildMetadata(selection, build, config)
	requireNPMDiagnostic(t, err, IDSourceRefInvalid)
}
