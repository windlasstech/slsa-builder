package npmprofile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/digest"
)

func TestFinalizeWorkflowBuildMetadata(t *testing.T) {
	repositoryRoot := t.TempDir()
	manifest := []byte(`{"name":"@windlass/slsa-builder","version":"1.2.3","repository":"https://github.com/example/project"}`)
	if err := os.WriteFile(filepath.Join(repositoryRoot, "package.json"), manifest, 0o600); err != nil {
		t.Fatal(err)
	}
	lockfile := []byte(`{"lockfileVersion":3}`)
	if err := os.WriteFile(filepath.Join(repositoryRoot, "package-lock.json"), lockfile, 0o600); err != nil {
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
		EventName: "push", RefType: "tag", Ref: "refs/tags/v1.2.3", Revision: testSourceSHA,
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
	if parameters.Package.Name != selection.Package.Name || parameters.Publish.ResolvedRegistryURL != "https://registry.npmjs.org/" ||
		parameters.Caller.WorkflowFilename != "release.yml" || parameters.Release.VersionTag != "v1.2.3" {
		t.Fatalf("unexpected external parameters: %#v", parameters)
	}
	if len(metadata.ResolvedDependencies) != 2 {
		t.Fatalf("resolved dependencies = %d, want lockfile and runner image", len(metadata.ResolvedDependencies))
	}
}

func TestFinalizeWorkflowBuildMetadataUsesBuiltSourceDuringDispatch(t *testing.T) {
	repositoryRoot := t.TempDir()
	manifest := []byte(`{"name":"pkg","version":"1.2.3","repository":"https://github.com/example/project"}`)
	if err := os.WriteFile(filepath.Join(repositoryRoot, "package.json"), manifest, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repositoryRoot, "package-lock.json"), []byte(`{"lockfileVersion":3}`), 0o600); err != nil {
		t.Fatal(err)
	}
	selection := Result{
		Package: Package{Directory: ".", RealDirectory: repositoryRoot, RealManagerRoot: repositoryRoot, ManagerRoot: ".", Name: "pkg", Version: "1.2.3", Repository: "https://github.com/example/project"},
		Manager: ManagerSelection{Name: ManagerNPM, Version: "11.5.1", Source: SelectionPackageManager, SelectionManifestPath: "package.json", SelectedLockfilePath: "package-lock.json"},
	}
	build := BuildPackResult{
		PackageName: "pkg", PackageVersion: "1.2.3", TarballPath: filepath.Join(repositoryRoot, "pkg-1.2.3.tgz"),
		SHA256: mustSHA256(t, testSHA256), SHA512: mustSHA512(t, testSHA512),
		Packed:      PackedMetadata{Name: "pkg", Version: "1.2.3"},
		BuildScript: BuildScriptCapture{Result: BuildScriptSkippedAbsent},
		Toolchain:   ToolchainCapture{NodeVersion: "v24.0.0", NPMVersion: "11.5.1", PackageManagerVersion: "11.5.1", Runner: RunnerCapture{ImageOS: "ubuntu24", ImageVersion: "20260801.1.0", IncludedSoftwareURL: "https://github.com/actions/runner-images/blob/main/images/ubuntu/Ubuntu2404-Readme.md"}},
	}
	metadata, err := FinalizeWorkflowBuildMetadata(selection, build, WorkflowBuildMetadataConfig{
		ArtifactName: "artifact", EventName: "workflow_dispatch", RefType: "tag",
		Ref: "refs/tags/v1.2.3", Revision: testSourceSHA, WorkflowSHA: testAttestSHA,
		CallerWorkflowFilename: "release.yml",
		RegistryState:          RegistryPreflightState{PackageExists: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	parameters, err := DecodeExternalParameters(metadata.ExternalParameters)
	if err != nil {
		t.Fatal(err)
	}
	if parameters.Source.Ref != "refs/tags/v1.2.3" || parameters.Source.Revision != testSourceSHA ||
		parameters.Source.EventName != "workflow_dispatch" || parameters.Source.RefType != "tag" {
		t.Fatalf("unexpected built source parameters: %#v", parameters.Source)
	}
}

func TestFinalizeWorkflowBuildMetadataRejectsGuardAndModeDrift(t *testing.T) {
	selection := Result{Package: Package{Directory: ".", Name: "pkg", Version: "1.2.3", Repository: "https://github.com/example/project"}}
	build := BuildPackResult{PackageName: "pkg", PackageVersion: "1.2.3"}
	base := WorkflowBuildMetadataConfig{EventName: "push", RefType: "tag", Ref: "refs/tags/v1.2.3", Revision: testSourceSHA, WorkflowSHA: testSourceSHA, CallerWorkflowFilename: "release.yml"}
	tests := map[string]func(*WorkflowBuildMetadataConfig){
		"branch ref":          func(config *WorkflowBuildMetadataConfig) { config.RefType = "branch" },
		"wrong tag":           func(config *WorkflowBuildMetadataConfig) { config.Ref = "refs/tags/v1.2.4" },
		"unsupported event":   func(config *WorkflowBuildMetadataConfig) { config.EventName = "pull_request" },
		"mutable builder ref": func(config *WorkflowBuildMetadataConfig) { config.WorkflowSHA = "main" },
		"release asset mode":  func(config *WorkflowBuildMetadataConfig) { config.ReleaseAssetMode = true },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			config := base
			mutate(&config)
			if _, err := FinalizeWorkflowBuildMetadata(selection, build, config); err == nil {
				t.Fatal("FinalizeWorkflowBuildMetadata() succeeded, want rejection")
			}
		})
	}
}

func TestResolvePublishIntent(t *testing.T) {
	manifest := PublishConfigParameters{Registry: "https://registry.npmjs.org/", Tag: "next", Access: "public"}
	intent, err := ResolvePublishIntent("", "", "", &manifest)
	if err != nil {
		t.Fatal(err)
	}
	if intent.ResolvedRegistryURL != "https://registry.npmjs.org/" || intent.ResolvedDistTag != "next" || intent.PublishAccessOption == nil || *intent.PublishAccessOption != "public" {
		t.Fatalf("unexpected publish intent: %#v", intent)
	}
	if _, err := ResolvePublishIntent("", "latest", "", &manifest); err == nil {
		t.Fatal("ResolvePublishIntent() accepted conflicting dist-tag")
	}
}

func mustSHA256(t *testing.T, value string) digest.SHA256 {
	t.Helper()
	parsed, err := digest.ParseSHA256(value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func mustSHA512(t *testing.T, value string) digest.SHA512 {
	t.Helper()
	parsed, err := digest.ParseSHA512(value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
