package npmprofile

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/provenance"
	"github.com/windlasstech/slsa-builder/internal/toolchainpins"
)

func TestWorkflowResolvedDependenciesOrder(t *testing.T) {
	t.Parallel()
	selection, build, config := workflowIntentInputs(t)
	selection.Package.SourceManifest.Content = []byte(`{"name":"@windlass/slsa-builder","version":"1.2.3","repository":"example/project","scripts":{"build":"true"}}`)
	metadata, err := FinalizeWorkflowBuildMetadata(selection, build, config)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, descriptor := range metadata.ResolvedDependencies {
		names = append(names, descriptor.Name)
	}
	want := []string{"invocation-source", "built-source", "package-manifest", "lockfile", "node-runtime", "runner-image"}
	if !slices.Equal(names, want) {
		t.Fatalf("dependency order = %v, want %v", names, want)
	}
	for _, index := range []int{2, 3} {
		if metadata.ResolvedDependencies[index].MediaType != "application/json" {
			t.Fatalf("descriptor %s has no JSON media type", names[index])
		}
	}
	if metadata.ResolvedDependencies[4].MediaType != "application/gzip" {
		t.Fatal("runtime media type must describe compressed bytes")
	}
	if _, present := metadata.ResolvedDependencies[5].Annotations["node_version"]; present {
		t.Fatal("runner must not duplicate node version")
	}
}

func TestFinalProducerSigning(t *testing.T) {
	t.Parallel()
	selection, build, config := workflowIntentInputs(t)
	selection.Package.SourceManifest.Content = []byte(`{"name":"@windlass/slsa-builder","version":"1.2.3","repository":"example/project"}`)
	build.BuildScript = BuildScriptCapture{Result: BuildScriptSkippedAbsent}
	pair, err := toolchainpins.NodeRuntimePair()
	if err != nil {
		t.Fatal(err)
	}
	build.Toolchain.NodeVersion = "v" + pair.NodeVersion
	build.Toolchain.NPMVersion = pair.NPMVersion
	metadata, err := FinalizeWorkflowBuildMetadata(selection, build, config)
	if err != nil {
		t.Fatal(err)
	}
	input := NPMProvenanceInput{
		BuildMetadata: metadata,
		BuilderID:     "https://github.com/windlasstech/slsa-builder/" + NPMWorkflowPath + "@" + testSourceSHA,
		GitHub:        provenance.GitHubParameters{EventName: "push", RepositoryID: "123", RepositoryOwnerID: "456"},
		NodeJSVersion: build.Toolchain.NodeVersion, NPMVersion: build.Toolchain.NPMVersion,
		InvocationRevision: testSourceSHA, Packed: build.Packed,
		InvocationID: "https://github.com/example/project/actions/runs/123456789/attempts/1",
		StartedOn:    "2026-08-06T12:00:00Z", FinishedOn: "2026-08-06T12:00:03Z",
		RuntimeReleaseRef: config.Ref, PeeledReleaseRevision: config.Revision,
	}
	input.BuildMetadata.StartedOn, input.BuildMetadata.FinishedOn = input.StartedOn, input.FinishedOn
	signing, err := NewProvenanceSigningInput(input)
	if err != nil {
		t.Fatal(err)
	}
	if signing.Predicate.RunDetails.Builder.Version["npm"] != pair.NPMVersion {
		t.Fatal("bundled npm observation missing")
	}
	if signing.Predicate.RunDetails.Metadata.BuildScriptDisposition != "skipped-absent" {
		t.Fatal("signed disposition missing")
	}
	statement := signing.Statement()
	statement.Predicate.RunDetails.Metadata.BuildScriptDisposition = "executed"
	requireNPMDiagnostic(t, ValidateNPMStatement(statement, input), IDBuildScriptDisposition)
}

func TestBuildScriptDisposition(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name    string
		script  string
		capture BuildScriptCapture
		valid   bool
	}{
		{"executed", `,"scripts":{"build":"true"}`, BuildScriptCapture{Present: true, Result: BuildScriptExecuted}, true},
		{"skipped", "", BuildScriptCapture{Result: BuildScriptSkippedAbsent}, true},
		{"declared but skipped", `,"scripts":{"build":"true"}`, BuildScriptCapture{Result: BuildScriptSkippedAbsent}, false},
		{"absent but executed", "", BuildScriptCapture{Present: true, Result: BuildScriptExecuted}, false},
		{"missing", "", BuildScriptCapture{}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			selection, build, config := workflowIntentInputs(t)
			selection.Package.SourceManifest.Content = []byte(`{"name":"@windlass/slsa-builder","version":"1.2.3","repository":"example/project"` + test.script + `}`)
			build.BuildScript = test.capture
			metadata, err := FinalizeWorkflowBuildMetadata(selection, build, config)
			if !test.valid {
				requireNPMDiagnostic(t, err, IDBuildScriptDisposition)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if metadata.BuildScriptDisposition != test.capture.Result {
				t.Fatalf("disposition = %q", metadata.BuildScriptDisposition)
			}
			encoded, err := json.Marshal(metadata)
			if err != nil {
				t.Fatal(err)
			}
			var decoded BuildMetadata
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatal(err)
			}
			if decoded.ManifestEvidence != metadata.ManifestEvidence {
				t.Fatal("lost original evidence in handoff")
			}
		})
	}
}
