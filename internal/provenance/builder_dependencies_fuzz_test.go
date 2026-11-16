package provenance_test

import (
	"encoding/json"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

func FuzzDecodeBuilderDependencies(f *testing.F) {
	f.Add([]byte("[" + signerDescriptorJSON + "]"))
	for _, test := range builderDependencyRejections() {
		f.Add([]byte(test.raw))
	}
	f.Add([]byte(`[{"name":"signing-adapter","\u006eame":"signing-adapter"}]`))
	f.Add([]byte(`[{"digest":{"dirHash1":"a","dirHash1":"b"}}]`))
	f.Fuzz(func(t *testing.T, raw []byte) {
		// Given arbitrary bytes at the existing Statement decoder's dependency boundary.
		input := append([]byte(`{"predicate":{"runDetails":{"builder":{"builderDependencies":`), raw...)
		input = append(input, []byte(`}}}}`)...)
		// When the boundary accepts a value, serialization must preserve its decoded meaning.
		statement, err := provenance.DecodeStatement(input)
		if err == nil {
			encoded, marshalErr := json.Marshal(statement.Predicate.RunDetails.Builder.BuilderDependencies)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			// Then all non-empty fields survive, so semantic validation cannot lose evidence.
			if canonicaljson.Validate(raw) == nil {
				var descriptors []map[string]json.RawMessage
				if json.Unmarshal(raw, &descriptors) == nil {
					var roundTrip []map[string]json.RawMessage
					if err := json.Unmarshal(encoded, &roundTrip); err != nil {
						t.Fatal(err)
					}
					for index, descriptor := range descriptors {
						if name, exists := descriptor["name"]; exists {
							var token string
							if json.Unmarshal(name, &token) != nil || token == "" {
								continue
							}
							equal, err := canonicaljson.Equal(name, roundTrip[index]["name"])
							if err != nil || !equal {
								t.Fatalf("descriptor name lost across decode/encode: input=%s output=%s", raw, encoded)
							}
						}
					}
				}
			}
		}
		duplicate := []byte(`{"predicate":{"runDetails":{"builder":{"builderDependencies":[],"\u0062uilderDependencies":` + string(raw) + `}}}}`)
		if canonicaljson.Validate(input) == nil {
			_, err := provenance.DecodeStatement(duplicate)
			requireDiagnosticID(t, err, "windlass.verify.error.duplicate-json-member")
		}
	})
}
