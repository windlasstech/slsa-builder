package provenance_test

import (
	"encoding/json"
	"maps"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

func FuzzDecodeBuilderDependencies(f *testing.F) {
	f.Add([]byte("[" + signerDescriptorJSON + "]"))
	f.Add([]byte("[" + admittedDescriptorJSON + "]"))
	f.Add([]byte(`[],"x":1`))
	f.Add([]byte(`[{"digest":{"sha256":null}}]`))
	f.Add([]byte(`[null,false,42,"text",[],[1],{"name":false,"uri":42,"digest":{"sha256":null},"mediaType":[1],"annotations":false,"unknown":{"x":2}}]`))
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
			// Then every effective member and non-object value survives serialization.
			var descriptors, roundTrip []json.RawMessage
			if err := json.Unmarshal(raw, &descriptors); err == nil {
				if err := json.Unmarshal(encoded, &roundTrip); err != nil {
					t.Fatalf("decode round-trip for raw %q: %v", raw, err)
				}
				if len(descriptors) != len(roundTrip) {
					t.Fatalf("descriptor count changed for raw %q: %d -> %d", raw, len(descriptors), len(roundTrip))
				}
				var decodedAgain []provenance.BuilderDependency
				if err := json.Unmarshal(encoded, &decodedAgain); err != nil {
					t.Fatalf("decode descriptors for raw %q: %v", raw, err)
				}
				original := statement.Predicate.RunDetails.Builder.BuilderDependencies
				if len(original) != len(decodedAgain) {
					t.Fatalf("decoded descriptor count changed for raw %q", raw)
				}
				for index, descriptor := range descriptors {
					want := descriptor
					var members map[string]json.RawMessage
					if json.Unmarshal(descriptor, &members) == nil && members != nil {
						for key, value := range members {
							canonical, err := canonicaljson.Canonicalize(value)
							if err != nil {
								t.Fatal(err)
							}
							switch string(canonical) {
							case "null", `""`, "[]", "{}":
								delete(members, key)
							}
						}
						var err error
						want, err = json.Marshal(members)
						if err != nil {
							t.Fatal(err)
						}
					}
					equal, err := canonicaljson.Equal(want, roundTrip[index])
					if err != nil || !equal {
						t.Fatalf("descriptor evidence changed: %s -> %s (%v)", descriptor, roundTrip[index], err)
					}
					before, after := original[index], decodedAgain[index]
					if before.Name != after.Name || before.URI != after.URI || before.MediaType != after.MediaType ||
						!maps.Equal(before.Digest, after.Digest) || before.HasMalformedMembers() != after.HasMalformedMembers() ||
						before.HasUnexpectedFields() != after.HasUnexpectedFields() {
						t.Fatalf("decoded descriptor changed: %#v -> %#v", before, after)
					}
					if len(before.Annotations) != 0 || len(after.Annotations) != 0 {
						equal, err := canonicaljson.Equal(before.Annotations, after.Annotations)
						if err != nil || !equal {
							t.Fatalf("annotations changed: %s -> %s (%v)", before.Annotations, after.Annotations, err)
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
