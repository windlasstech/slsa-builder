package fixture

import (
	"encoding/json"
	"errors"
	"testing"
)

func FuzzFixtureManifestEvidence(f *testing.F) {
	statement, evidence := evidenceFixtureBytes(f)
	registryManifest, err := json.Marshal(validAcceptedManifest())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(registryManifest, []byte{}, false)
	f.Add(statement, evidence, true)
	f.Add(statement, []byte(`{"path":"packages/a/package.json","content_base64":"e31="}`), true)
	f.Add(statement, []byte(`null`), false)
	f.Add([]byte(`{"phase":"build-pack"}`), []byte{}, false)
	f.Fuzz(func(t *testing.T, document, rawEvidence []byte, hasPath bool) {
		manifest := validAcceptedManifest()
		if hasPath {
			manifest.ManifestEvidence = stringPointer("testdata/npm/evidence.json")
		}
		first := ValidateManifestEvidence(manifest, document, rawEvidence)
		second := ValidateManifestEvidence(manifest, document, rawEvidence)
		if (first == nil) != (second == nil) {
			t.Fatal("evidence validation is nondeterministic")
		}
		if first != nil {
			var identified interface{ DiagnosticID() string }
			if !errors.As(first, &identified) || identified.DiagnosticID() != IDPackageManifestEvidence {
				t.Fatalf("unclassified evidence rejection: %v", first)
			}
		}
		if rejectDuplicateMembers(document) == nil {
			if decoded, err := decodeManifest(document); err == nil {
				encoded, err := json.Marshal(decoded)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := decodeManifest(encoded); err != nil {
					t.Fatalf("accepted registry manifest did not round-trip: %v", err)
				}
			}
		}
	})
}
