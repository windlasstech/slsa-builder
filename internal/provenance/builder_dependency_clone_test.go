package provenance

import (
	"encoding/json"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/canonicaljson"
)

func TestCloneBuilderDependenciesIsolatesEvidence(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{`[1]`, `{"name":false,"unknown":[1],"annotations":{"x":2},"digest":{"sha256":"ab"}}`} {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()
			// Given retained evidence whose private storage is cloned for profile callbacks.
			var original BuilderDependency
			if err := json.Unmarshal([]byte(raw), &original); err != nil {
				t.Fatal(err)
			}
			cloned := cloneBuilderDependencies([]BuilderDependency{original})
			// When a callback mutates every mutable part of its copy.
			clear(cloned[0].invalidObject)
			clear(cloned[0].Annotations)
			for _, value := range cloned[0].malformed {
				clear(value)
			}
			for _, value := range cloned[0].unexpected {
				clear(value)
			}
			clear(cloned[0].Digest)
			// Then the caller's entire original descriptor is unchanged.
			encoded, err := json.Marshal(original)
			if err != nil {
				t.Fatal(err)
			}
			equal, err := canonicaljson.Equal(encoded, []byte(raw))
			if err != nil || !equal {
				t.Fatalf("clone mutation changed evidence: %s (%v)", encoded, err)
			}
		})
	}
}
