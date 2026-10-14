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
	for _, github := range []bool{false, true} {
		t.Run(map[bool]string{false: "stdout", true: "github output append"}[github], func(t *testing.T) {
			t.Parallel()
			// Given a successful domain operation and optional existing GitHub output.
			directory := t.TempDir()
			binDir := filepath.Join(directory, "node", "bin")
			githubOutput := filepath.Join(directory, "outputs")
			args := []string{"npm-profile-provision-node", "--output-directory", directory}
			if github {
				if err := os.WriteFile(githubOutput, []byte("existing=value\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				args = append(args, "--github-output", githubOutput)
			}
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
			// When dispatched through the real command boundary.
			result := NewDispatcher(adapter).Dispatch(t.Context(), args, &out)
			// Then success emits only the bin directory and preserves existing outputs.
			if result.ExitCode != 0 || out.String() != binDir+"\n" {
				t.Fatalf("result = %+v, output = %q", result, out.String())
			}
			if github {
				got, err := os.ReadFile(githubOutput)
				if err != nil || string(got) != "existing=value\nnode-bin="+binDir+"\n" {
					t.Fatalf("github output = %q, %v", got, err)
				}
			}
		})
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

func TestProvisionNodeCommandReportPathSuccess(t *testing.T) {
	t.Parallel()
	// Given a successful provisioning and a requested report path.
	directory := t.TempDir()
	reportPath := filepath.Join(directory, "outcome.json")
	adapter := npmProfileProvisionNodeCommand{provision: func(context.Context, string) (string, error) {
		return filepath.Join(directory, "node", "bin"), nil
	}}
	var out bytes.Buffer
	// When dispatched with --report-path.
	result := NewDispatcher(adapter).Dispatch(t.Context(), []string{adapter.Name(), "--output-directory", directory, "--report-path", reportPath}, &out)
	// Then success persists no report file.
	if result.ExitCode != 0 {
		t.Fatalf("exit = %d, output = %s", result.ExitCode, out.Bytes())
	}
	if _, err := os.Stat(reportPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("report path exists after success: %v", err)
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
