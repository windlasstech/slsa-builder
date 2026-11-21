package npmprofile

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"
)

func builderContractVariants(t *testing.T, spec contractFixtureSpec) []contractFixtureVariant {
	t.Helper()
	var variants []contractFixtureVariant
	for index, edit := range builderContractEdits(spec.mutation) {
		// Given the real producer's valid two-descriptor Statement.
		variant := contractVariant(t, spec.manager, spec.scenario)
		statement := decodeContractStatement(t, variant.Statement)
		original := mustJSON(statement.Predicate.RunDetails.Builder.BuilderDependencies)
		var descriptors []map[string]json.RawMessage
		if err := json.Unmarshal(original, &descriptors); err != nil {
			t.Fatal(err)
		}
		if len(builderContractEdits(spec.mutation)) > 1 {
			variant.Name = fmt.Sprintf("%s-%d-%s", spec.mutation, index, edit.field)
		}
		if spec.mutation == "builder-annotations" || spec.mutation == "builder-signer-annotations" {
			variant.Name = "annotations-" + edit.value
		}
		// When applying the stated mutation, keep unrelated evidence intact.
		switch spec.mutation {
		case "builder-reordered":
			descriptors[0], descriptors[1] = descriptors[1], descriptors[0]
		case "builder-unknown":
			descriptors = append(descriptors, map[string]json.RawMessage{"name": mustJSON("unknown"), "uri": mustJSON("pkg:npm/unknown@1.0.0"), "digest": mustJSON(map[string]string{"sha256": testSHA256})})
		case "builder-common-shape":
			descriptors = append(descriptors, map[string]json.RawMessage{"name": mustJSON(42)})
		case "builder-legacy-digest":
			hash, err := hex.DecodeString(statement.Predicate.RunDetails.Builder.BuilderDependencies[0].Digest["dirHash1"])
			if err != nil {
				t.Fatal(err)
			}
			descriptors[0]["digest"] = mustJSON(map[string]string{"h1": base64.StdEncoding.EncodeToString(hash)})
		case "builder-missing":
			descriptors = descriptors[:1]
		case "builder-duplicate":
			descriptors = append(descriptors, descriptors[1])
		case "builder-signer-missing":
			descriptors = descriptors[1:]
		case "builder-signer-duplicate":
			descriptors = append(descriptors, descriptors[0])
		case "builder-publish-precedence", "builder-signer-precedence":
			descriptors[0]["uri"] = mustJSON("pkg:golang/github.com/sigstore/sigstore-go@v0.0.0")
			if spec.mutation == "builder-publish-precedence" {
				descriptors[1]["uri"] = mustJSON("pkg:npm/npm@0.0.0")
			}
			descriptors = append(descriptors, map[string]json.RawMessage{"name": mustJSON(42)})
		default:
			if edit.field == "" {
				t.Fatalf("unknown builder mutation %q", spec.mutation)
			}
			delete(descriptors[edit.descriptor], edit.field)
			if edit.value != "" {
				descriptors[edit.descriptor][edit.field] = json.RawMessage(edit.value)
			}
		}
		if bytes.Count(variant.Statement, original) != 1 {
			t.Fatal("expected one builder dependency array")
		}
		variant.Statement = bytes.Replace(variant.Statement, original, mustJSON(descriptors), 1)
		if spec.mutation == "builder-duplicate-json" {
			variant.StatementJSON = string(variant.Statement)
			variant.Statement = nil
		}
		// Then the catalog's primary ID is checked by TestFinalContractCorpus.
		variants = append(variants, variant)
	}
	return variants
}
