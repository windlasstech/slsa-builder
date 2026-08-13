package npmprofile

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNPMProvenanceInputSourceRefDispatchRetryGolden(t *testing.T) {
	input := sourceRefDispatchRetryProvenanceInput(t)
	signing, err := NewProvenanceSigningInput(input)
	if err != nil {
		t.Fatalf("NewProvenanceSigningInput() error = %v", err)
	}
	goldenPath := filepath.Join("..", "..", "testdata", "npm", "provenance", "npm-predicate-source-ref-dispatch-retry.jcs.json")
	if os.Getenv("UPDATE_P02_GOLDEN") == "1" {
		if err := os.WriteFile(goldenPath, signing.PredicateJSON, 0o600); err != nil {
			t.Fatalf("write generated golden: %v", err)
		}
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden predicate: %v", err)
	}
	if !bytes.Equal(signing.PredicateJSON, want) {
		t.Fatalf("predicate differs from source-ref JCS golden\n got: %s\nwant: %s", signing.PredicateJSON, want)
	}
	parameters, err := DecodeExternalParameters(signing.Predicate.BuildDefinition.ExternalParameters)
	if err != nil {
		t.Fatal(err)
	}
	if parameters.Source.InputRef == nil || *parameters.Source.InputRef != "refs/tags/v1.2.3" ||
		parameters.Source.InvocationRef == nil || *parameters.Source.InvocationRef != "refs/heads/main" ||
		parameters.Source.InvocationRevision == nil || *parameters.Source.InvocationRevision != testAttestSHA {
		t.Fatalf("source invocation record = %#v", parameters.Source)
	}
}

func TestNPMProvenanceInputSourceRefOmittedByteCompatibility(t *testing.T) {
	input := validProvenanceInput(t, ManagerPNPM)
	signing, err := NewProvenanceSigningInput(input)
	if err != nil {
		t.Fatalf("NewProvenanceSigningInput() error = %v", err)
	}
	want, err := os.ReadFile(filepath.Join("..", "..", "testdata", "npm", "provenance", "npm-predicate.jcs.json"))
	if err != nil {
		t.Fatalf("read existing golden predicate: %v", err)
	}
	if !bytes.Equal(signing.PredicateJSON, want) {
		t.Fatalf("omitted source-ref changed existing predicate bytes\n got: %s\nwant: %s", signing.PredicateJSON, want)
	}
	var external map[string]json.RawMessage
	if err := json.Unmarshal(signing.Predicate.BuildDefinition.ExternalParameters, &external); err != nil {
		t.Fatal(err)
	}
	var source map[string]json.RawMessage
	if err := json.Unmarshal(external["source"], &source); err != nil {
		t.Fatal(err)
	}
	for _, member := range []string{"input_ref", "invocation_ref", "invocation_revision"} {
		if _, present := source[member]; present {
			t.Errorf("source.%s is present, want absent", member)
		}
	}
}

func TestNPMProvenanceInputSourceRefRejections(t *testing.T) {
	validInvocation := map[string]json.RawMessage{
		"input_ref":           mustJSON("refs/tags/v1.2.3"),
		"invocation_ref":      mustJSON("refs/heads/main"),
		"invocation_revision": mustJSON(testAttestSHA),
	}
	tests := []struct {
		name   string
		wantID string
		mutate func(map[string]json.RawMessage)
	}{
		{name: "input ref differs from built source", wantID: IDSourceRefInvalid, mutate: func(source map[string]json.RawMessage) {
			copySourceMembers(source, validInvocation)
			source["input_ref"] = mustJSON("refs/tags/v1.2.4")
		}},
		{name: "invocation members without input ref", wantID: IDUnexpectedExternalParameters, mutate: func(source map[string]json.RawMessage) {
			source["invocation_ref"] = validInvocation["invocation_ref"]
			source["invocation_revision"] = validInvocation["invocation_revision"]
		}},
		{name: "input ref without invocation ref", wantID: IDUnexpectedExternalParameters, mutate: func(source map[string]json.RawMessage) {
			copySourceMembers(source, validInvocation)
			delete(source, "invocation_ref")
		}},
		{name: "input ref without invocation revision", wantID: IDUnexpectedExternalParameters, mutate: func(source map[string]json.RawMessage) {
			copySourceMembers(source, validInvocation)
			delete(source, "invocation_revision")
		}},
		{name: "null input ref", wantID: IDUnexpectedExternalParameters, mutate: func(source map[string]json.RawMessage) {
			copySourceMembers(source, validInvocation)
			source["input_ref"] = json.RawMessage(`null`)
		}},
		{name: "null invocation ref", wantID: IDUnexpectedExternalParameters, mutate: func(source map[string]json.RawMessage) {
			copySourceMembers(source, validInvocation)
			source["invocation_ref"] = json.RawMessage(`null`)
		}},
		{name: "null invocation revision", wantID: IDUnexpectedExternalParameters, mutate: func(source map[string]json.RawMessage) {
			copySourceMembers(source, validInvocation)
			source["invocation_revision"] = json.RawMessage(`null`)
		}},
		{name: "malformed invocation ref", wantID: IDUnexpectedExternalParameters, mutate: func(source map[string]json.RawMessage) {
			copySourceMembers(source, validInvocation)
			source["invocation_ref"] = mustJSON("main")
		}},
		{name: "malformed invocation SHA", wantID: IDUnexpectedExternalParameters, mutate: func(source map[string]json.RawMessage) {
			copySourceMembers(source, validInvocation)
			source["invocation_revision"] = mustJSON("89ABCDEF")
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validProvenanceInput(t, ManagerPNPM)
			input.BuildMetadata.ExternalParameters = mutateSourceGroup(t, input.BuildMetadata.ExternalParameters, test.mutate)
			requireNPMDiagnostic(t, newProvenanceInputError(input), test.wantID)
		})
	}
}

func sourceRefDispatchRetryProvenanceInput(t *testing.T) NPMProvenanceInput {
	t.Helper()
	input := validProvenanceInput(t, ManagerPNPM)
	parameters := validExternalParameters(ManagerPNPM)
	parameters.Source.EventName = "workflow_dispatch"
	parameters.Source.InputRef = testStringPointer("refs/tags/v1.2.3")
	parameters.Source.InvocationRef = testStringPointer("refs/heads/main")
	parameters.Source.InvocationRevision = testStringPointer(testAttestSHA)
	encoded, err := EncodeExternalParameters(parameters)
	if err != nil {
		t.Fatalf("EncodeExternalParameters() error = %v", err)
	}
	input.BuildMetadata.ExternalParameters = encoded
	input.BuildMetadata.ResolvedDependencies = validDependencies(ManagerPNPM, parameters)
	return input
}

func mutateSourceGroup(t *testing.T, encoded json.RawMessage, mutate func(map[string]json.RawMessage)) json.RawMessage {
	t.Helper()
	var external map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &external); err != nil {
		t.Fatal(err)
	}
	var source map[string]json.RawMessage
	if err := json.Unmarshal(external["source"], &source); err != nil {
		t.Fatal(err)
	}
	mutate(source)
	external["source"] = mustJSON(source)
	mutated, err := json.Marshal(external)
	if err != nil {
		t.Fatal(err)
	}
	return mutated
}

func copySourceMembers(destination, source map[string]json.RawMessage) {
	for name, value := range source {
		destination[name] = append(json.RawMessage(nil), value...)
	}
}
