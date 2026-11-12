package npmprofile

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/provenance"
)

func contractVariants(t *testing.T, spec contractFixtureSpec) []contractFixtureVariant {
	t.Helper()
	if spec.mutation == "matrix" {
		return []contractFixtureVariant{contractVariant(t, ManagerNPM, "tag"), contractVariant(t, ManagerPNPM, "tag"), contractVariant(t, ManagerYarn, "tag")}
	}
	if spec.mutation == "diagnostics" {
		var variants []contractFixtureVariant
		for index, value := range []string{`{"package_identity_preexisting":true,"package_version_preexisting":false}`, `{"package_identity_preexisting":null,"package_version_preexisting":null}`, `{"package_identity_preexisting":true,"package_version_preexisting":true}`} {
			variant := contractVariant(t, spec.manager, spec.scenario)
			variant.Name = []string{"absent-version", "inconclusive", "existing-version"}[index]
			variant.DiagnosticJSON = `{"publish":` + value + `}`
			variants = append(variants, variant)
		}
		return variants
	}
	if spec.mutation == "removed" {
		var variants []contractFixtureVariant
		for _, field := range contractRemovedFields() {
			variant := contractVariant(t, spec.manager, spec.scenario)
			statement := decodeContractStatement(t, variant.Statement)
			statement.Predicate.BuildDefinition.ExternalParameters = mutateContractExternal(t, statement.Predicate.BuildDefinition.ExternalParameters, field, json.RawMessage(`null`))
			variant.Statement = mustJSON(statement)
			variant.Name = field
			variants = append(variants, variant)
		}
		return variants
	}
	variant := contractVariant(t, spec.manager, spec.scenario)
	statement := decodeContractStatement(t, variant.Statement)
	if operation, role, found := strings.Cut(spec.mutation, ":"); found {
		values := statement.Predicate.BuildDefinition.ResolvedDependencies
		index := slices.IndexFunc(values, func(d provenance.ResourceDescriptor) bool { return d.Name == role })
		if index < 0 {
			t.Fatalf("missing mutation role %q", role)
		}
		switch operation {
		case "missing":
			values = slices.Delete(values, index, index+1)
		case "duplicate":
			values = append(values, values[index])
		case "commit":
			values[index].Digest["gitCommit"] = strings.Repeat("c", 40)
		case "media":
			values[index].MediaType = "text/plain"
		default:
			t.Fatalf("unknown role mutation %q", spec.mutation)
		}
		statement.Predicate.BuildDefinition.ResolvedDependencies = values
	} else {
		switch spec.mutation {
		case "":
		case "caller-path":
			statement.Predicate.BuildDefinition.ExternalParameters = mutateContractExternal(t, statement.Predicate.BuildDefinition.ExternalParameters, "workflow.path", mustJSON(NPMWorkflowPath))
		case "builder-id":
			statement.Predicate.RunDetails.Builder.ID = "https://github.com/windlasstech/slsa-builder/" + NPMWorkflowPath + "@refs/heads/main"
		case "classifier":
			for index, d := range statement.Predicate.BuildDefinition.ResolvedDependencies {
				if d.Name == "built-source" {
					statement.Predicate.BuildDefinition.ResolvedDependencies[index].Annotations["ref_type"] = mustJSON("branch")
				}
			}
		case "selector-conflict":
			statement.Predicate.BuildDefinition.ExternalParameters = mutateContractExternal(t, statement.Predicate.BuildDefinition.ExternalParameters, "source.ref", mustJSON("refs/tags/v9.9.9"))
		case "selector-whitespace":
			statement.Predicate.BuildDefinition.ExternalParameters = mutateContractExternal(t, statement.Predicate.BuildDefinition.ExternalParameters, "source.ref", mustJSON(" refs/tags/v1.2.3 "))
		case "sidecar":
			statement.Predicate.BuildDefinition.ExternalParameters = mutateContractExternal(t, statement.Predicate.BuildDefinition.ExternalParameters, "distribution.provenance_sidecar", mustJSON("required"))
		case "event", "actor":
			var internal map[string]map[string]json.RawMessage
			if err := json.Unmarshal(statement.Predicate.BuildDefinition.InternalParameters, &internal); err != nil {
				t.Fatal(err)
			}
			if spec.mutation == "event" {
				internal["github"]["event_name"] = mustJSON("workflow_dispatch")
			} else {
				internal["github"]["actor_id"] = mustJSON("123")
			}
			statement.Predicate.BuildDefinition.InternalParameters = mustJSON(internal)
		case "manifest-path":
			for index, d := range statement.Predicate.BuildDefinition.ResolvedDependencies {
				if d.Name == "package-manifest" {
					statement.Predicate.BuildDefinition.ResolvedDependencies[index].URI += ".wrong"
				}
			}
		case "evidence-missing":
			variant.EvidenceJSON = ""
		case "evidence-base64":
			variant.EvidenceJSON = `{"path":"package.json","content_base64":"e31="}`
		case "evidence-digest":
			variant.EvidenceJSON = `{"path":"package.json","content_base64":"e30="}`
		case "evidence-path":
			variant.EvidenceJSON = strings.Replace(variant.EvidenceJSON, `"path":"package.json"`, `"path":"other/package.json"`, 1)
		case "evidence-duplicate":
			variant.EvidenceJSON = strings.TrimSuffix(variant.EvidenceJSON, "}") + `,"path":"package.json"}`
		case "disposition-missing":
			statement.Predicate.RunDetails.Metadata.BuildScriptDisposition = ""
		case "disposition-mismatch":
			statement.Predicate.RunDetails.Metadata.BuildScriptDisposition = "skipped-absent"
		case "npm-missing":
			delete(statement.Predicate.RunDetails.Builder.Version, "npm")
		default:
			t.Fatalf("unknown contract mutation %q", spec.mutation)
		}
	}
	variant.Statement = mustJSON(statement)
	return []contractFixtureVariant{variant}
}

func mutateContractExternal(t *testing.T, raw json.RawMessage, field string, value json.RawMessage) json.RawMessage {
	t.Helper()
	var external map[string]json.RawMessage
	if err := json.Unmarshal(raw, &external); err != nil {
		t.Fatal(err)
	}
	group, member, nested := strings.Cut(field, ".")
	if !nested {
		external[group] = value
		return mustJSON(external)
	}
	object := map[string]json.RawMessage{}
	if existing, ok := external[group]; ok {
		if err := json.Unmarshal(existing, &object); err != nil {
			t.Fatal(err)
		}
	}
	if group == "source" {
		object["ref"] = mustJSON("refs/tags/v1.2.3")
	}
	object[member] = value
	external[group] = mustJSON(object)
	return mustJSON(external)
}

func decodeContractStatement(t *testing.T, raw []byte) provenance.Statement {
	t.Helper()
	statement, err := provenance.DecodeStatement(raw)
	if err != nil {
		t.Fatal(err)
	}
	return statement
}
