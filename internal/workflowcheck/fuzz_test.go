package workflowcheck

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/goccy/go-yaml"
)

// FuzzDecodeWorkflow fuzzes decodeWorkflow over arbitrary bytes. The decoder
// sits on the trust boundary (workflow YAML is attacker-controlled input per
// docs/testing-guide.md), so the target asserts invariants rather than only
// "did it panic":
//
//   - determinism: two decodes of the same bytes agree, in error state and in
//     the resulting document;
//   - round-trip: a successfully decoded document re-marshals with
//     yaml.Marshal and the re-marshaled bytes decode again without error.
func FuzzDecodeWorkflow(f *testing.F) {
	// Repository workflow fixtures, the same files the package's lint test
	// loads from .github/workflows.
	for _, name := range []string{
		"lint.yml",
		"js-ts-npm-package-slsa3.yml",
		"autofix.yml",
		"dependency-review.yml",
		"osv-scanner.yml",
		"scorecard.yml",
	} {
		contents, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", name))
		if err != nil {
			f.Fatalf("read workflow seed %q: %v", name, err)
		}
		f.Add(contents)
	}

	// The build, provenance-sign, and publish workflow fixtures shared with
	// check_test.go.
	f.Add([]byte(validBuildWorkflow))
	f.Add([]byte(validSigningWorkflow))
	f.Add([]byte(validNPMOnlyWorkflow))

	// Malformed and adversarial YAML literals.
	f.Add([]byte("{{{"))
	f.Add([]byte("\tkey: value"))
	f.Add([]byte("key: [unclosed"))
	f.Add([]byte("permissions:\n  contents: read\npermissions:\n  contents: write\n"))
	f.Add([]byte("permissions: null"))
	f.Add([]byte("jobs:\n  build:\n    steps: [{uses: a/b@c"))
	f.Add([]byte("anchor: &a [x, x, x]\nalias: [*a, *a, *a]"))
	f.Add([]byte("%YAML 1.1\n---\nyes: no\n...\n---\nsecond: document\n"))
	f.Add([]byte(""))
	f.Add([]byte("null"))
	f.Add([]byte("- just\n- a\n- sequence\n"))
	f.Add([]byte{0x00, 0x01, 0xff, 0xfe, 0x7f})
	f.Add([]byte("on:\n  workflow_call:\n    inputs:\n      package-directory: {required: true, type: string}\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		first, firstErr := decodeWorkflow(data)
		second, secondErr := decodeWorkflow(data)
		if (firstErr == nil) != (secondErr == nil) {
			t.Fatalf("decode is not deterministic: first error %v, second error %v", firstErr, secondErr)
		}
		if firstErr != nil {
			return
		}
		if !reflect.DeepEqual(first, second) {
			t.Fatal("decode is not deterministic: two decodes produced different documents")
		}

		remarshaled, err := yaml.Marshal(first)
		if err != nil {
			t.Fatalf("re-marshal decoded workflow: %v", err)
		}
		if _, err := decodeWorkflow(remarshaled); err != nil {
			t.Fatalf("re-decode re-marshaled workflow: %v\nremarshaled bytes: %q", err, remarshaled)
		}
	})
}
