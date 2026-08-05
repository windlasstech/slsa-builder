package command

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/digest"
)

func TestVerifyHandoffCommand(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	artifactDirectory := filepath.Join(directory, "artifact")
	if err := os.Mkdir(artifactDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	payload := []byte("verified payload")
	if err := os.WriteFile(filepath.Join(artifactDirectory, "artifact.bin"), payload, 0o600); err != nil {
		t.Fatal(err)
	}
	contractPath := filepath.Join(directory, "handoff.json")
	contract := `{"transport":"github-actions-artifact","artifact_name":"artifact-1","payload_file_name":"artifact.bin","payload_kind":"primary-artifact","digest":{"algorithm":"sha256","value":"` + digest.SumSHA256(payload).String() + `"}}`
	if err := os.WriteFile(contractPath, []byte(contract), 0o600); err != nil {
		t.Fatal(err)
	}
	outputPath := filepath.Join(directory, "verified.bin")
	githubOutputPath := filepath.Join(directory, "github-output")
	var output bytes.Buffer
	result := NewDispatcher(NewVerifyHandoffCommand()).Dispatch(context.Background(), []string{
		"verify-handoff",
		"--handoff", contractPath,
		"--artifact-dir", artifactDirectory,
		"--output", outputPath,
		"--github-output", githubOutputPath,
	}, &output)
	if result.ExitCode != ExitCodeSuccess {
		t.Fatalf("exit code = %d; output=%s", result.ExitCode, output.String())
	}
	written, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(written, payload) {
		t.Fatalf("verified output = %q", written)
	}
	var report Report
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if report.Result != "pass" || report.ExitCode != 0 {
		t.Fatalf("report = %#v", report)
	}
}
