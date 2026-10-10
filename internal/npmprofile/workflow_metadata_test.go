package npmprofile

import (
	"encoding/json"
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

// TestFinalizeWorkflowBuildMetadataDescriptorRendering covers ADR 0094:
// package_manager.version renders the selected pnpm or Yarn descriptor
// verbatim, including the declared +<algorithm>.<hex> suffix, and the
// distribution annotation records the same descriptor reference.
func TestFinalizeWorkflowBuildMetadataDescriptorRendering(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		manager       Manager
		version       string
		authority     string
		lockfile      string
		lockfileBytes []byte
		distribution  string
	}{
		{name: "pnpm", manager: ManagerPNPM, version: "11.9.0", authority: "registry-integrity", lockfile: "pnpm-lock.yaml", lockfileBytes: []byte("lockfileVersion: '9.0'\n"), distribution: "https://registry.npmjs.org/pnpm/-/pnpm-11.9.0.tgz"},
		{name: "yarn", manager: ManagerYarn, version: "4.9.2", authority: "download-hash", lockfile: "yarn.lock", lockfileBytes: []byte("__metadata:\n  version: 8\n"), distribution: "https://repo.yarnpkg.com/4.9.2/packages/yarnpkg-cli/bin/yarn.js"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repositoryRoot := t.TempDir()
			manifest := []byte(`{"name":"@windlass/slsa-builder","version":"1.2.3","repository":"https://github.com/example/project"}`)
			if err := os.WriteFile(filepath.Join(repositoryRoot, "package.json"), manifest, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(repositoryRoot, test.lockfile), test.lockfileBytes, 0o600); err != nil {
				t.Fatal(err)
			}
			descriptor := test.version + "+sha512." + testSHA512
			selection := Result{
				Package: Package{Directory: ".", RealDirectory: repositoryRoot, RealManagerRoot: repositoryRoot, ManagerRoot: ".", Name: "@windlass/slsa-builder", Version: "1.2.3", Repository: "https://github.com/example/project"},
				Manager: ManagerSelection{Name: test.manager, Version: test.version, Digest: &DeclaredDigest{Algorithm: "sha512", Hex: testSHA512}, Source: SelectionPackageManager, SelectionManifestPath: "package.json", SelectedLockfilePath: test.lockfile},
			}
			build := BuildPackResult{
				PackageName: "@windlass/slsa-builder", PackageVersion: "1.2.3",
				TarballPath: filepath.Join(repositoryRoot, "windlass-slsa-builder-1.2.3.tgz"),
				SHA256:      mustSHA256(t, testSHA256), SHA512: mustSHA512(t, testSHA512),
				Packed:      PackedMetadata{Name: "@windlass/slsa-builder", Version: "1.2.3", Files: []string{"package.json"}},
				BuildScript: BuildScriptCapture{Present: true, Result: BuildScriptExecuted},
				Toolchain: ToolchainCapture{
					NodeVersion: "v24.0.0", NPMVersion: "11.5.1", PackageManagerVersion: test.version,
					Distribution: &DistributionCapture{
						URL: test.distribution, SHA512: testSHA512, DigestAuthority: test.authority,
						PackageManager: test.manager, PackageManagerVer: descriptor, AcquisitionSource: "corepack",
					},
					Runner: RunnerCapture{ImageOS: "ubuntu24", ImageVersion: "20260801.1.0", IncludedSoftwareURL: "https://github.com/actions/runner-images/blob/main/images/ubuntu/Ubuntu2404-Readme.md"},
				},
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
			if parameters.PackageManager.Version != descriptor {
				t.Fatalf("package_manager.version = %q, want verbatim descriptor %q", parameters.PackageManager.Version, descriptor)
			}
			var annotated string
			for _, dependency := range metadata.ResolvedDependencies {
				if dependency.Name != "package-manager-distribution" {
					continue
				}
				if err := json.Unmarshal(dependency.Annotations["package_manager_version"], &annotated); err != nil {
					t.Fatal(err)
				}
			}
			if annotated != descriptor {
				t.Fatalf("package_manager_version annotation = %q, want %q", annotated, descriptor)
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
