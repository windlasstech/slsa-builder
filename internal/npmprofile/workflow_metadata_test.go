package npmprofile

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/digest"
)

func TestFinalizeWorkflowBuildMetadata(t *testing.T) {
	selection, build := workflowMetadataFixture(t)
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

func TestFinalizeWorkflowBuildMetadataDispatchRetry(t *testing.T) {
	selection, build := workflowMetadataFixture(t)
	invocationSHA := "89abcdef0123456789abcdef0123456789abcdef"
	metadata, err := FinalizeWorkflowBuildMetadata(selection, build, WorkflowBuildMetadataConfig{
		ArtifactName: "js-ts-npm-package-tarball-123456789-1", RegistryURLInput: "https://registry.npmjs.org/",
		EventName: "workflow_dispatch", RefType: "branch", Ref: "refs/heads/main", Revision: testSourceSHA,
		SourceRefInput: "refs/tags/v1.2.3", InvocationRef: "refs/heads/main", InvocationRevision: invocationSHA,
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
	source := parameters.Source
	if source.Ref != "refs/tags/v1.2.3" || source.Revision != testSourceSHA || source.RefType != "tag" || source.EventName != "workflow_dispatch" {
		t.Fatalf("built source identity = %#v", source)
	}
	if source.InputRef == nil || *source.InputRef != "refs/tags/v1.2.3" {
		t.Fatalf("input_ref = %v, want the supplied source-ref", source.InputRef)
	}
	if source.InvocationRef == nil || *source.InvocationRef != "refs/heads/main" ||
		source.InvocationRevision == nil || *source.InvocationRevision != invocationSHA {
		t.Fatalf("invocation record = %v/%v, want the dispatch context", source.InvocationRef, source.InvocationRevision)
	}
	if parameters.Release.Ref != "refs/tags/v1.2.3" || parameters.Release.VersionTag != "v1.2.3" {
		t.Fatalf("release identity = %#v, want the built tag", parameters.Release)
	}
}

func TestFinalizeWorkflowBuildMetadataOmittedSourceRefKeepsSingleIdentity(t *testing.T) {
	selection, build := workflowMetadataFixture(t)
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
	if parameters.Source.InputRef != nil || parameters.Source.InvocationRef != nil || parameters.Source.InvocationRevision != nil {
		t.Fatalf("single-identity source group carries an invocation record: %#v", parameters.Source)
	}
	var sourceGroup map[string]json.RawMessage
	if err := json.Unmarshal(metadata.ExternalParameters, &struct {
		Source *map[string]json.RawMessage `json:"source"`
	}{Source: &sourceGroup}); err != nil {
		t.Fatal(err)
	}
	for _, member := range []string{"input_ref", "invocation_ref", "invocation_revision"} {
		if _, present := sourceGroup[member]; present {
			t.Fatalf("single-identity source group must omit %q", member)
		}
	}
}

func TestFinalizeWorkflowBuildMetadataRejectsSourceRefDrift(t *testing.T) {
	selection, build := workflowMetadataFixture(t)
	invocationSHA := "89abcdef0123456789abcdef0123456789abcdef"
	base := WorkflowBuildMetadataConfig{
		ArtifactName: "js-ts-npm-package-tarball-123456789-1", EventName: "workflow_dispatch",
		RefType: "branch", Ref: "refs/heads/main", Revision: testSourceSHA,
		SourceRefInput: "refs/tags/v1.2.3", InvocationRef: "refs/heads/main", InvocationRevision: invocationSHA,
		WorkflowSHA: testSourceSHA, CallerWorkflowFilename: "release.yml",
	}
	tests := map[string]func(*WorkflowBuildMetadataConfig){
		"short tag name": func(config *WorkflowBuildMetadataConfig) { config.SourceRefInput = "v1.2.3" },
		"branch ref":     func(config *WorkflowBuildMetadataConfig) { config.SourceRefInput = "refs/heads/v1.2.3" },
		"commit sha":     func(config *WorkflowBuildMetadataConfig) { config.SourceRefInput = testSourceSHA },
		"version mismatch": func(config *WorkflowBuildMetadataConfig) {
			config.SourceRefInput = "refs/tags/v1.2.4"
		},
		"invocation tag conflict": func(config *WorkflowBuildMetadataConfig) {
			config.RefType = "tag"
			config.Ref = "refs/tags/v1.2.3"
			config.SourceRefInput = "refs/tags/v1.2.4"
		},
		"missing invocation record": func(config *WorkflowBuildMetadataConfig) {
			config.InvocationRef = ""
			config.InvocationRevision = ""
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			config := base
			mutate(&config)
			_, err := FinalizeWorkflowBuildMetadata(selection, build, config)
			if err == nil {
				t.Fatal("FinalizeWorkflowBuildMetadata() succeeded, want rejection")
			}
			var diagnosticError *npmProvenanceValidationError
			if name == "missing invocation record" {
				if errors.As(err, &diagnosticError) {
					t.Fatalf("workflow wiring failure must not use a caller diagnostic, got %v", err)
				}
				return
			}
			if !errors.As(err, &diagnosticError) || diagnosticError.DiagnosticID() != IDSourceRefInvalid {
				t.Fatalf("error = %v, want diagnostic %q", err, IDSourceRefInvalid)
			}
		})
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

func workflowMetadataFixture(t *testing.T) (Result, BuildPackResult) {
	t.Helper()
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
	return selection, build
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
