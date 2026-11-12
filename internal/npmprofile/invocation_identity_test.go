package npmprofile

import (
	"encoding/json"
	"testing"
)

func TestNPMWorkflowInvocationParameters(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		workflow map[string]json.RawMessage
		wantID   string
	}{
		{
			name: "caller invocation accepted",
			workflow: map[string]json.RawMessage{
				"ref":        mustJSON("refs/tags/v1.2.3"),
				"repository": mustJSON("https://github.com/example/project"),
				"path":       mustJSON(".github/workflows/release.yml"),
			},
		},
		{
			name: "missing invocation ref rejected",
			workflow: map[string]json.RawMessage{
				"repository": mustJSON("https://github.com/example/project"),
				"path":       mustJSON(".github/workflows/release.yml"),
			},
			wantID: IDUnexpectedExternalParameters,
		},
		{
			name: "missing caller repository rejected",
			workflow: map[string]json.RawMessage{
				"ref":  mustJSON("refs/tags/v1.2.3"),
				"path": mustJSON(".github/workflows/release.yml"),
			},
			wantID: IDUnexpectedExternalParameters,
		},
		{
			name: "missing caller path rejected",
			workflow: map[string]json.RawMessage{
				"ref":        mustJSON("refs/tags/v1.2.3"),
				"repository": mustJSON("https://github.com/example/project"),
			},
			wantID: IDUnexpectedExternalParameters,
		},
		{
			name: "removed builder SHA rejected",
			workflow: map[string]json.RawMessage{
				"ref":        mustJSON("refs/tags/v1.2.3"),
				"repository": mustJSON("https://github.com/example/project"),
				"path":       mustJSON(".github/workflows/release.yml"),
				"sha":        mustJSON(testSourceSHA),
			},
			wantID: IDUnexpectedExternalParameters,
		},
		{
			name: "removed builder ID rejected",
			workflow: map[string]json.RawMessage{
				"ref":        mustJSON("refs/tags/v1.2.3"),
				"repository": mustJSON("https://github.com/example/project"),
				"path":       mustJSON(".github/workflows/release.yml"),
				"builder_id": mustJSON("https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@" + testSourceSHA),
			},
			wantID: IDUnexpectedExternalParameters,
		},
		{
			name: "reusable builder path is not caller path",
			workflow: map[string]json.RawMessage{
				"ref":        mustJSON("refs/tags/v1.2.3"),
				"repository": mustJSON("https://github.com/example/project"),
				"path":       mustJSON(NPMWorkflowPath),
			},
			wantID: "windlass.verify.error.build-config-uri-mismatch",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Given valid build metadata with the case's workflow invocation group.
			input := validProvenanceInput(t, ManagerNPM)
			input.BuildMetadata.ExternalParameters = replaceWorkflowGroup(t, input.BuildMetadata.ExternalParameters, test.workflow)

			// When the producer assembles and validates the signing input.
			_, err := NewProvenanceSigningInput(input)

			// Then exactly the caller {ref, repository, path} group is accepted.
			if test.wantID == "" {
				if err != nil {
					t.Fatalf("NewProvenanceSigningInput() error = %v", err)
				}
				return
			}
			requireNPMDiagnostic(t, err, test.wantID)
		})
	}
}

func TestNPMDispatchRetryWorkflowRefPreservesInvocation(t *testing.T) {
	t.Parallel()

	// Given a dispatch retry that builds a caller-selected release tag from main.
	input := sourceRefDispatchRetryProvenanceInput(t)
	input.BuildMetadata.ExternalParameters = replaceWorkflowGroup(t, input.BuildMetadata.ExternalParameters, map[string]json.RawMessage{
		"ref":        mustJSON("refs/heads/main"),
		"repository": mustJSON("https://github.com/example/project"),
		"path":       mustJSON(".github/workflows/release.yml"),
	})

	// When the dispatch-retry signing input is assembled.
	signing, err := NewProvenanceSigningInput(input)
	if err != nil {
		t.Fatalf("NewProvenanceSigningInput() error = %v", err)
	}

	// Then workflow records the invocation ref while source fields retain the built tag.
	var external map[string]json.RawMessage
	if err := json.Unmarshal(signing.Predicate.BuildDefinition.ExternalParameters, &external); err != nil {
		t.Fatal(err)
	}
	var workflow map[string]json.RawMessage
	if err := json.Unmarshal(external["workflow"], &workflow); err != nil {
		t.Fatal(err)
	}
	if got := string(workflow["ref"]); got != `"refs/heads/main"` {
		t.Fatalf("workflow.ref = %s, want invocation ref refs/heads/main", got)
	}
	var source map[string]json.RawMessage
	if err := json.Unmarshal(external["source"], &source); err != nil {
		t.Fatal(err)
	}
	if got := string(source["ref"]); got != `"refs/tags/v1.2.3"` {
		t.Fatalf("source.ref = %s, want built release tag refs/tags/v1.2.3", got)
	}
	if len(source) != 1 {
		t.Fatalf("source must contain only the caller selector: %s", external["source"])
	}
}

func TestNPMBuilderMigrationGuard(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		builderID string
		wantID    string
	}{
		{
			name:      "different well-formed Windlass filename rejected",
			builderID: "https://github.com/windlasstech/slsa-builder/.github/workflows/other-windlass.yml@" + testSourceSHA,
			wantID:    "windlass.verify.error.builder-id-not-immutable",
		},
		{
			name:      "well-formed but unauthorized builder SHA rejected",
			builderID: "https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@" + testAttestSHA,
			wantID:    "windlass.verify.error.signer-workflow-sha-mismatch",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Given an otherwise valid statement and an independent authorized signer expectation.
			config, signing := publishVerifyInputs(t)
			statement := signing.Statement()
			statement.Predicate.RunDetails.Builder.ID = test.builderID
			verifier := &sigstorePublishVerifier{config: config}

			// When npm semantic verification evaluates the candidate builder identity.
			err := verifier.validateStatement(statement)

			// Then generic well-formedness cannot bypass the exact path or authorized SHA.
			requireNPMDiagnostic(t, err, test.wantID)
		})
	}
}

func replaceWorkflowGroup(t *testing.T, encoded json.RawMessage, workflow map[string]json.RawMessage) json.RawMessage {
	t.Helper()
	var external map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &external); err != nil {
		t.Fatal(err)
	}
	external["workflow"] = mustJSON(workflow)
	mutated, err := json.Marshal(external)
	if err != nil {
		t.Fatal(err)
	}
	return mutated
}
