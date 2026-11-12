package npmprofile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFinalizeWorkflowBuildMetadataSourceRefDispatchRetry(t *testing.T) {
	repositoryRoot := t.TempDir()
	manifest := testOriginalManifest()
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
	selection.Package.SourceManifest = SourceManifestSnapshot{Path: "package.json", Content: manifest}
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
		WorkflowSHA: testSourceSHA, CallerWorkflowPath: ".github/workflows/release.yml", CallerWorkflowFilename: "release.yml",
		RegistryState: RegistryPreflightState{PackageExists: true},
		Repository:    "https://github.com/example/project",
	})
	if err != nil {
		t.Fatal(err)
	}
	parameters, err := DecodeExternalParameters(metadata.ExternalParameters)
	if err != nil {
		t.Fatal(err)
	}
	if parameters.Source.Ref != "refs/tags/v1.2.3" || metadata.ResolvedDependencies[1].Digest["gitCommit"] != testSourceSHA || string(metadata.ResolvedDependencies[1].Annotations["ref_type"]) != `"tag"` {
		t.Fatalf("built source = %#v", parameters.Source)
	}
	if parameters.Workflow.Ref != "refs/heads/main" || metadata.ResolvedDependencies[0].Digest["gitCommit"] != testAttestSHA {
		t.Fatalf("invocation record = %#v", metadata.ResolvedDependencies[0])
	}
	if effectiveBuiltRef(parameters) != "refs/tags/v1.2.3" {
		t.Fatalf("release = %q", effectiveBuiltRef(parameters))
	}
	if got := metadata.ResolvedDependencies[3].URI; got != "git+https://github.com/example/project@"+testSourceSHA+"#package-lock.json" {
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
	manifest := testOriginalManifest()
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
	selection.Package.SourceManifest = SourceManifestSnapshot{Path: "package.json", Content: manifest}
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
		WorkflowSHA: testSourceSHA, CallerWorkflowPath: ".github/workflows/release.yml", CallerWorkflowFilename: "release.yml",
		RegistryState: RegistryPreflightState{PackageExists: true},
		Repository:    "https://github.com/example/project",
	}
	config.SourceRefInput = " \t\n\r\v\f"
	_, err := FinalizeWorkflowBuildMetadata(selection, build, config)
	requireNPMDiagnostic(t, err, IDSourceRefInvalid)
}
