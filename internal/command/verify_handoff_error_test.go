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
	if result.ExitCode != 1 {
		t.Fatalf("dispatch exit code = %d, want 1", result.ExitCode)
	}
	assertPrimaryDiagnostic(t, output.Bytes(), 1, "windlass.verify.error.handoff-schema-mismatch")
}

func assertPrimaryDiagnostic(t *testing.T, output []byte, wantExit int, wantID string) {
	t.Helper()
	var report Report
	if err := json.Unmarshal(output, &report); err != nil {
		t.Fatalf("decode report: %v; output=%s", err, output)
	}
	if report.ExitCode != wantExit {
		t.Fatalf("report exit code = %d, want %d; output=%s", report.ExitCode, wantExit, output)
	}
	if report.PrimaryID == nil || *report.PrimaryID != wantID {
		t.Fatalf("primary ID = %v, want %s", report.PrimaryID, wantID)
	}
}
