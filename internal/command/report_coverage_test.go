package command

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/diagnostic"
)

func TestReportCommandRecoversUnavailableReport(t *testing.T) {
	t.Parallel()
	for _, malformed := range []bool{false, true} {
		name := "missing"
		if malformed {
			name = "malformed"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "report.json")
			if malformed {
				if err := os.WriteFile(path, []byte(`{`), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			githubOutput := filepath.Join(t.TempDir(), "out")
			var output bytes.Buffer
			err := NewNPMProfileReportCommand().Execute(t.Context(), []string{"--report-path", path, "--github-output", githubOutput}, &output)
			if err != nil {
				t.Fatal(err)
			}
			assertPrimaryDiagnostic(t, output.Bytes(), 2, 2, diagnostic.IDInputUnavailable)
			if !bytes.Equal(mustRead(t, path), bytes.TrimSpace(output.Bytes())) {
				t.Fatal("persisted report differs from emitted report")
			}
			if got := parseGitHubOutputs(t, string(mustRead(t, githubOutput)))["result"]; got != "fail" {
				t.Fatalf("result = %q, want fail", got)
			}
		})
	}
}

func TestReportCommandOutputFailures(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"report file", "github output", "report writer", "invalid report"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "report.json")
			githubOutput := filepath.Join(t.TempDir(), "out")
			report, err := diagnostic.Build(nil, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := report.CanonicalJSON()
			if err != nil {
				t.Fatal(err)
			}
			if stage == "invalid report" {
				encoded = []byte(`{}`)
			}
			if err := os.WriteFile(path, encoded, 0o600); err != nil {
				t.Fatal(err)
			}
			switch stage {
			case "report file":
				path = t.TempDir()
			case "github output":
				githubOutput = t.TempDir()
			}
			var output bytes.Buffer
			cause := errors.New("report sink failed")
			args := []string{"--report-path", path, "--github-output", githubOutput}
			if stage == "report writer" {
				err = NewNPMProfileReportCommand().Execute(t.Context(), args, &stagedWriter{err: cause})
				if !errors.Is(err, cause) {
					t.Fatalf("error = %v, want writer cause", err)
				}
				return
			}
			err = NewNPMProfileReportCommand().Execute(t.Context(), args, &output)
			if err == nil || errors.Is(err, ErrInvocationFailure) || errors.Is(err, ErrVerificationFailure) || output.Len() != 0 {
				t.Fatalf("output failure = %v, output %q", err, output.String())
			}
		})
	}
}
