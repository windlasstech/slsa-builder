package command

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/diagnostic"
	"github.com/windlasstech/slsa-builder/internal/digest"
)

func TestHandoffCommandClassifiesLocalFailures(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"missing contract", "invalid contract", "output directory", "github output directory"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			payload := []byte("verified payload")
			artifact := oneFileDirectory(t, "artifact.bin", payload)
			contractPath := filepath.Join(t.TempDir(), "handoff.json")
			contract := `{"transport":"github-actions-artifact","artifact_name":"artifact-1","payload_file_name":"artifact.bin","payload_kind":"primary-artifact","digest":{"algorithm":"sha256","value":"` + digest.SumSHA256(payload).String() + `"}}`
			if stage == "invalid contract" {
				contract = `{}`
			}
			if stage != "missing contract" {
				if err := os.WriteFile(contractPath, []byte(contract), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			outputPath := filepath.Join(t.TempDir(), "verified.bin")
			githubOutput := ""
			if stage == "output directory" {
				outputPath = t.TempDir()
			}
			if stage == "github output directory" {
				githubOutput = t.TempDir()
			}
			var output bytes.Buffer
			err := NewVerifyHandoffCommand().Execute(t.Context(), []string{"--handoff", contractPath, "--artifact-dir", artifact, "--output", outputPath, "--github-output", githubOutput}, &output)
			wantErr, code, id := ErrInvocationFailure, 2, diagnostic.IDVerifierExecutionFailure
			switch stage {
			case "missing contract":
				id = diagnostic.IDInputUnavailable
			case "invalid contract":
				wantErr, code, id = ErrVerificationFailure, 1, "windlass.verify.error.handoff-schema-mismatch"
			}
			if !errors.Is(err, wantErr) {
				t.Fatalf("Execute() = %v, want %v", err, wantErr)
			}
			assertPrimaryDiagnostic(t, output.Bytes(), code, id)
		})
	}
}
