package npmprofile

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

func builderContractVariants(t *testing.T, spec contractFixtureSpec) []contractFixtureVariant {
	t.Helper()
	annotationValues := []string{""}
	if spec.mutation == "builder-annotations" {
		annotationValues = []string{"", "null", "false", "0", `"untrusted"`, `[]`, `["untrusted"]`, `{"digest_authority":"untrusted","other":true}`}
	}
	var variants []contractFixtureVariant
	for _, annotation := range annotationValues {
		// Given the real producer's valid two-descriptor Statement.
		variant := contractVariant(t, spec.manager, spec.scenario)
		statement := decodeContractStatement(t, variant.Statement)
		original := mustJSON(statement.Predicate.RunDetails.Builder.BuilderDependencies)
		var descriptors []map[string]json.RawMessage
		if err := json.Unmarshal(original, &descriptors); err != nil {
			t.Fatal(err)
		}
		// When applying one isolated descriptor mutation, keep all other evidence intact.
		switch spec.mutation {
		case "builder-reordered":
			descriptors[0], descriptors[1] = descriptors[1], descriptors[0]
		case "builder-annotations":
			variant.Name = "annotations-" + annotation
			delete(descriptors[1], "annotations")
			if annotation != "" {
				descriptors[1]["annotations"] = json.RawMessage(annotation)
			}
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
		case "builder-digest":
			descriptors[1]["digest"] = mustJSON(map[string]string{"sha512": strings.Repeat("0", 128)})
		case "builder-extra":
			descriptors[1]["extra"] = json.RawMessage(`false`)
		default:
			t.Fatalf("unknown builder mutation %q", spec.mutation)
		}
		if bytes.Count(variant.Statement, original) != 1 {
			t.Fatal("expected one builder dependency array")
		}
		variant.Statement = bytes.Replace(variant.Statement, original, mustJSON(descriptors), 1)
		// Then the catalog's primary ID is checked by TestFinalContractCorpus.
		variants = append(variants, variant)
	}
	return variants
}
