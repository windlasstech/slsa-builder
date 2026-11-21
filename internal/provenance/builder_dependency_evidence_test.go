package provenance_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

func TestBuilderDependencyDigestSizes(t *testing.T) {
	t.Parallel()
	for algorithm, size := range map[string]int{"sha1": 40, "sha256": 64, "sha384": 96, "sha512": 128} {
		for _, test := range []struct {
			name, value string
			valid       bool
		}{
			{"exact", strings.Repeat("ab", size/2), true},
			{"short", strings.Repeat("a", size-2), false},
			{"long", strings.Repeat("a", size+2), false},
			{"odd", strings.Repeat("a", size-1), false},
			{"nonhex", strings.Repeat("z", size), false},
		} {
			t.Run(algorithm+"/"+test.name, func(t *testing.T) {
				t.Parallel()
				// Given a profile-admitted descriptor with one digest under test.
				raw := `[{"name":"publish-cli","uri":"pkg:generic/tool@1","digest":{"` + algorithm + `":"` + test.value + `"}},` + signerDescriptorJSON + `]`
				statement, err := decodeDependencyStatement(t, raw)
				if err != nil {
					t.Fatal(err)
				}
				// When common shape validation runs.
				_, err = statement.Validate(expectations(nil), builderDependencyProfile{})
				// Then only an exact-size lowercase hex digest is accepted.
				if test.valid {
					if err != nil {
						t.Fatal(err)
					}
				} else {
					requireDiagnosticID(t, err, descriptorInvalidID)
				}
			})
		}
	}
}

func TestBuilderDependencyUnknownDigestPolicy(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, value string
		valid       bool
	}{
		{"documented encoding", "not-hex/base64=", true},
		{"empty encoding", "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			statement, err := decodeDependencyStatement(t, `[{"name":"publish-cli","uri":"pkg:generic/tool@1","digest":{"custom":"`+test.value+`"}},`+signerDescriptorJSON+`]`)
			if err != nil {
				t.Fatal(err)
			}
			_, err = statement.Validate(expectations(nil), builderDependencyProfile{})
			if test.valid {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				requireDiagnosticID(t, err, descriptorInvalidID)
			}
		})
	}
}

func TestBuilderDependencyRetainsEvidence(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{`null`, `false`, `42`, `"text"`, `[ 1, {"x":2} ]`, `[]`, `{"name":false,"uri":23,"digest":{"sha256":3},"mediaType":[1],"annotations":{"x":2},"unknown":[false]}`} {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()
			// Given invalid descriptor evidence owned initially by the input buffer.
			input := []byte(raw)
			var dependency provenance.BuilderDependency
			if err := dependency.UnmarshalJSON(input); err != nil {
				t.Fatal(err)
			}
			// When the caller reuses its buffer and the descriptor is serialized.
			clear(input)
			encoded, err := dependency.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			// Then rejected values retain their complete semantic evidence.
			equal, err := canonicaljson.Equal(encoded, []byte(raw))
			if err != nil || !equal {
				t.Fatalf("evidence changed: %s -> %s (%v)", raw, encoded, err)
			}
			if !dependency.HasMalformedMembers() {
				t.Fatal("malformed evidence lost")
			}
			if raw[0] != '{' && !bytes.Equal(encoded, []byte(raw)) {
				t.Fatalf("raw non-object changed: %s", encoded)
			}
			// Returning bytes must not expose retained storage to mutation.
			clear(encoded)
			again, err := json.Marshal(dependency)
			if err != nil {
				t.Fatal(err)
			}
			equal, err = canonicaljson.Equal(again, []byte(raw))
			if err != nil || !equal {
				t.Fatalf("marshal exposed evidence storage: %s (%v)", again, err)
			}
		})
	}
}
