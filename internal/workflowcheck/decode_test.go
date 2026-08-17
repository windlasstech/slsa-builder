// Regression tests for the goccy/go-yaml v1.19.2 TagNode.ArrayRange nil-iterator
// panic: when a tagged scalar is decoded into a slice field, ArrayRange returns a
// nil *ArrayNodeIter and Decoder.decodeSlice dereferences it.
//
// The pinned module revision (goccy/go-yaml#862) makes such input an ordinary
// decode error, and the decodeWorkflow panic guard is defense in depth.
// The contract locked by these tests: attacker-controlled workflow YAML must
// always produce an error, never a panic.
package workflowcheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecodeWorkflowTaggedScalarIntoSlice(t *testing.T) {
	tests := []string{
		"jobs:\n  build:\n    steps: !!str x\n",
		"jobs:\n  build:\n    steps: !foo bar\n",
	}

	for _, input := range tests {
		firstErr, secondErr := callDecodeWorkflow(t, []byte(input)), callDecodeWorkflow(t, []byte(input))

		if (firstErr == nil) != (secondErr == nil) {
			t.Fatalf("decodeWorkflow returned inconsistent success/failure for %q", input)
		}
		for _, err := range []error{firstErr, secondErr} {
			if err == nil {
				t.Fatalf("decodeWorkflow() = nil, want error for %q", input)
			}
			if !strings.Contains(err.Error(), "decode workflow:") {
				t.Fatalf("decodeWorkflow() error = %q, want to contain %q", err.Error(), "decode workflow:")
			}
		}
	}
}

func TestCheckBuildJobDecoderPanicIsError(t *testing.T) {
	contents := []byte("jobs:\n  build:\n    steps: !!str x\n")
	path := filepath.Join(t.TempDir(), "workflow.yml")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := CheckBuildJob(path)
	if err == nil {
		t.Fatal("CheckBuildJob() = nil, want error")
	}
	if !strings.Contains(err.Error(), "decode workflow:") {
		t.Fatalf("CheckBuildJob() error = %q, want to contain %q", err.Error(), "decode workflow:")
	}
}

func callDecodeWorkflow(t *testing.T, encoded []byte) error {
	t.Helper()
	_, err := decodeWorkflow(encoded)
	return err
}
