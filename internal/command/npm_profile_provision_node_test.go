package command

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/npmprofile"
)

func TestProvisionNodeCommandOutputs(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	binDir := filepath.Join(directory, "node", "bin")
	githubOutput := filepath.Join(directory, "outputs")
	adapter := npmProfileProvisionNodeCommand{provision: func(ctx context.Context, got string) (string, error) {
		if err := ctx.Err(); err != nil {
			t.Fatal(err)
		}
		if got != directory {
			t.Fatalf("output directory = %q", got)
		}
		return binDir, nil
	}}
	var out bytes.Buffer
	result := NewDispatcher(adapter).Dispatch(t.Context(), []string{adapter.Name(), "--output-directory", directory, "--github-output", githubOutput}, &out)
	if result.ExitCode != 0 || out.String() != binDir+"\n" {
		t.Fatalf("result = %+v, output = %q", result, out.String())
	}
	got, err := os.ReadFile(githubOutput)
	if err != nil || string(got) != "node-bin="+binDir+"\n" {
		t.Fatalf("github output = %q, %v", got, err)
	}
}

func TestProvisionNodeCommandInvocationFailures(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{nil, {"--unknown"}, {"--output-directory", "somewhere", "positional"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			t.Parallel()
			adapter := npmProfileProvisionNodeCommand{provision: func(context.Context, string) (string, error) {
				t.Fatal("invalid invocation reached provisioning")
				return "", errors.New("unreachable")
			}}
			var out bytes.Buffer
			result := NewDispatcher(adapter).Dispatch(t.Context(), append([]string{adapter.Name()}, args...), &out)
			if result.ExitCode != 2 {
				t.Fatalf("exit = %d", result.ExitCode)
			}
			assertPrimaryDiagnostic(t, out.Bytes(), 2, "windlass.verify.error.verifier-execution-failure")
		})
	}
}

func TestProvisionNodeCommandFailureReportAndPersistence(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	directory := t.TempDir()
	reportPath := filepath.Join(directory, "report.json")
	adapter := npmProfileProvisionNodeCommand{provision: func(context.Context, string) (string, error) {
		return npmprofile.ProvisionNode(ctx, directory)
	}}
	var out bytes.Buffer
	result := NewDispatcher(adapter).Dispatch(t.Context(), []string{adapter.Name(), "--output-directory", directory, "--report-path", reportPath}, &out)
	if result.ExitCode != 1 {
		t.Fatalf("exit = %d, output = %s", result.ExitCode, out.Bytes())
	}
	assertPrimaryDiagnostic(t, out.Bytes(), 1, "windlass.verify.error.node-runtime-provision-download")
	persisted, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(reportPath)
	if err != nil {
		t.Fatalf("stat report: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("report mode = %v", info.Mode())
	}
	if !bytes.Equal(persisted, bytes.TrimSpace(out.Bytes())) {
		t.Fatalf("persisted report differs from stdout: %q != %q", persisted, out.Bytes())
	}
}

func TestProvisionNodeCommandUnexpectedFailure(t *testing.T) {
	t.Parallel()
	adapter := npmProfileProvisionNodeCommand{provision: func(context.Context, string) (string, error) {
		return "", errors.New("transport exploded")
	}}
	var out bytes.Buffer
	result := NewDispatcher(adapter).Dispatch(t.Context(), []string{adapter.Name(), "--output-directory", t.TempDir()}, &out)
	if result.ExitCode != 2 {
		t.Fatalf("exit = %d, output = %s", result.ExitCode, out.Bytes())
	}
	assertPrimaryDiagnostic(t, out.Bytes(), 2, "windlass.verify.error.verifier-execution-failure")
}

func TestProvisionNodeCommandHelp(t *testing.T) {
	t.Parallel()
	adapter := npmProfileProvisionNodeCommand{}
	var out bytes.Buffer
	if err := adapter.Execute(t.Context(), []string{"--help"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Usage: slsa-builder-internal npm-profile-provision-node --output-directory <dir>") {
		t.Fatalf("help = %q", out.String())
	}
}
