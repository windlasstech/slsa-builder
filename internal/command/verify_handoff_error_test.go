package command

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyHandoffRejectsMalformedContract(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	contractPath := filepath.Join(directory, "handoff.json")
	if err := os.WriteFile(contractPath, []byte(`{"unexpected":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	result := NewDispatcher(NewVerifyHandoffCommand()).Dispatch(context.Background(), []string{
		"verify-handoff", "--handoff", contractPath, "--artifact-dir", directory, "--output", filepath.Join(directory, "output"),
	}, &output)
	assertPrimaryDiagnostic(t, output.Bytes(), result.ExitCode, 1, "windlass.verify.error.handoff-schema-mismatch")
}

func assertPrimaryDiagnostic(t *testing.T, output []byte, gotExit, wantExit int, wantID string) {
	t.Helper()
	if gotExit != wantExit {
		t.Fatalf("exit code = %d, want %d; output=%s", gotExit, wantExit, output)
	}
	var report Report
	if err := json.Unmarshal(output, &report); err != nil {
		t.Fatalf("decode report: %v; output=%s", err, output)
	}
	if report.PrimaryID == nil || *report.PrimaryID != wantID {
		t.Fatalf("primary ID = %v, want %s", report.PrimaryID, wantID)
	}
}
