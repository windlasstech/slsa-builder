package command

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/npmprofile"
)

func TestProvisionNPMCommandOutputs(t *testing.T) {
	t.Parallel()
	for _, github := range []bool{false, true} {
		t.Run(map[bool]string{false: "stdout", true: "github output append"}[github], func(t *testing.T) {
			t.Parallel()
			// Given a successful domain operation and optional existing GitHub output.
			directory := t.TempDir()
			launcher := filepath.Join(directory, "npm")
			githubOutput := filepath.Join(directory, "outputs")
			args := []string{"npm-profile-provision-npm", "--output-directory", directory}
			if github {
				if err := os.WriteFile(githubOutput, []byte("existing=value\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				args = append(args, "--github-output", githubOutput)
			}
			adapter := npmProfileProvisionNPMCommand{provision: func(ctx context.Context, got string) (string, error) {
				if err := ctx.Err(); err != nil {
					t.Fatal(err)
				}
				if got != directory {
					t.Fatalf("directory = %q", got)
				}
				return launcher, nil
			}}
			var out bytes.Buffer
			// When dispatched through the real command boundary.
			result := NewDispatcher(adapter).Dispatch(t.Context(), args, &out)
			// Then success emits only the launcher and preserves existing outputs.
			if result.ExitCode != 0 || out.String() != launcher+"\n" {
				t.Fatalf("result = %+v, output = %q", result, out.String())
			}
			if github {
				got, err := os.ReadFile(githubOutput)
				if err != nil || string(got) != "existing=value\nnpm-executable="+launcher+"\n" {
					t.Fatalf("github output = %q, %v", got, err)
				}
			}
		})
	}
}

func TestProvisionNPMCommandInvocationFailures(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{nil, {"--unknown"}, {"--output-directory", "somewhere", "positional"}} {
		t.Run("args", func(t *testing.T) {
			t.Parallel()
			// Given invalid invocation arguments.
			adapter := npmProfileProvisionNPMCommand{provision: func(context.Context, string) (string, error) {
				t.Fatal("invalid invocation reached provisioning")
				return "", errors.New("unreachable")
			}}
			var out bytes.Buffer
			// When dispatched.
			result := NewDispatcher(adapter).Dispatch(t.Context(), append([]string{adapter.Name()}, args...), &out)
			// Then invocation failures produce a canonical exit-2 report.
			if result.ExitCode != 2 {
				t.Fatalf("exit = %d", result.ExitCode)
			}
			assertPrimaryDiagnostic(t, out.Bytes(), 2, "windlass.verify.error.verifier-execution-failure")
		})
	}
}

func TestProvisionNPMCommandDownloadFailureReport(t *testing.T) {
	t.Parallel()
	// Given a canceled domain request, which must never use the network.
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	adapter := npmProfileProvisionNPMCommand{provision: func(invocationContext context.Context, directory string) (string, error) {
		if err := invocationContext.Err(); err != nil {
			t.Fatal(err)
		}
		return npmprofile.ProvisionNPM(ctx, directory)
	}}
	var out bytes.Buffer
	// When provisioning fails through the adapter.
	result := NewDispatcher(adapter).Dispatch(t.Context(), []string{adapter.Name(), "--output-directory", t.TempDir()}, &out)
	// Then a canonical verification report is the final output.
	if result.ExitCode != 1 {
		t.Fatalf("exit = %d, output = %s", result.ExitCode, out.Bytes())
	}
	assertPrimaryDiagnostic(t, out.Bytes(), 1, "windlass.verify.error.npm-provision-download")
}

func TestProvisionNPMCommandUnexpectedFailure(t *testing.T) {
	t.Parallel()
	// Given a domain failure that is not a classified provisioning error.
	adapter := npmProfileProvisionNPMCommand{provision: func(context.Context, string) (string, error) {
		return "", errors.New("transport exploded")
	}}
	var out bytes.Buffer
	// When dispatched.
	result := NewDispatcher(adapter).Dispatch(t.Context(), []string{adapter.Name(), "--output-directory", t.TempDir()}, &out)
	// Then the unclassified cause becomes a canonical invocation report with exit 2.
	if result.ExitCode != 2 {
		t.Fatalf("exit = %d, output = %s", result.ExitCode, out.Bytes())
	}
	assertPrimaryDiagnostic(t, out.Bytes(), 2, "windlass.verify.error.verifier-execution-failure")
}

func TestProvisionNPMCommandUnregisteredDiagnostic(t *testing.T) {
	t.Parallel()
	// Given a provisioning failure carrying an identity outside the closed registry.
	adapter := npmProfileProvisionNPMCommand{provision: func(context.Context, string) (string, error) {
		return "", &npmprofile.NPMProvisionError{}
	}}
	var out bytes.Buffer
	// When dispatched.
	result := NewDispatcher(adapter).Dispatch(t.Context(), []string{adapter.Name(), "--output-directory", t.TempDir()}, &out)
	// Then the adapter refuses to emit a malformed report and fails as an invocation error.
	if result.ExitCode != 2 {
		t.Fatalf("exit = %d, output = %s", result.ExitCode, out.Bytes())
	}
	assertPrimaryDiagnostic(t, out.Bytes(), 2, "windlass.verify.error.verifier-execution-failure")
}

func TestProvisionNPMCommandReportWriteFailure(t *testing.T) {
	t.Parallel()
	// Given a classified provisioning failure and a sink that rejects the report.
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	adapter := npmProfileProvisionNPMCommand{provision: func(invocationContext context.Context, directory string) (string, error) {
		if err := invocationContext.Err(); err != nil {
			t.Fatal(err)
		}
		return npmprofile.ProvisionNPM(ctx, directory)
	}}
	cause := errors.New("report sink failed")
	// When the verification report cannot be written.
	err := adapter.Execute(t.Context(), []string{"--output-directory", t.TempDir()}, &stagedWriter{err: cause})
	// Then the sink cause surfaces instead of the verification-failure sentinel.
	if !errors.Is(err, cause) || errors.Is(err, ErrVerificationFailure) {
		t.Fatalf("Execute() = %v, want sink cause without report sentinel", err)
	}
}

func TestProvisionNPMCommandGitHubOutputFailure(t *testing.T) {
	t.Parallel()
	// Given a successful provisioning and a GitHub output path that cannot be opened.
	directory := t.TempDir()
	adapter := npmProfileProvisionNPMCommand{provision: func(context.Context, string) (string, error) {
		return filepath.Join(directory, "npm"), nil
	}}
	var out bytes.Buffer
	missing := filepath.Join(directory, "missing", "outputs")
	// When dispatched with an unwritable GitHub output target.
	result := NewDispatcher(adapter).Dispatch(t.Context(), []string{adapter.Name(), "--output-directory", directory, "--github-output", missing}, &out)
	// Then the output failure is reported as an invocation failure.
	if result.ExitCode != 2 {
		t.Fatalf("exit = %d, output = %s", result.ExitCode, out.Bytes())
	}
	assertPrimaryDiagnostic(t, out.Bytes(), 2, "windlass.verify.error.verifier-execution-failure")
}

func TestProvisionNPMCommandReportPath(t *testing.T) {
	t.Parallel()
	// Given a typed provisioning failure that never reaches the network.
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	directory := t.TempDir()
	reportPath := filepath.Join(directory, "outcome.json")
	adapter := npmProfileProvisionNPMCommand{provision: func(invocationContext context.Context, output string) (string, error) {
		if err := invocationContext.Err(); err != nil {
			t.Fatal(err)
		}
		return npmprofile.ProvisionNPM(ctx, output)
	}}
	var out bytes.Buffer
	// When the failure is dispatched with --report-path.
	result := NewDispatcher(adapter).Dispatch(t.Context(), []string{adapter.Name(), "--output-directory", directory, "--report-path", reportPath}, &out)
	// Then the specific failure report is also persisted at the path.
	if result.ExitCode != 1 {
		t.Fatalf("exit = %d, output = %s", result.ExitCode, out.Bytes())
	}
	persisted, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("persisted report: %v", err)
	}
	assertPrimaryDiagnostic(t, persisted, 1, "windlass.verify.error.npm-provision-download")
}

func TestProvisionNPMCommandReportPathSuccess(t *testing.T) {
	t.Parallel()
	// Given a successful provisioning and a requested report path.
	directory := t.TempDir()
	reportPath := filepath.Join(directory, "outcome.json")
	adapter := npmProfileProvisionNPMCommand{provision: func(context.Context, string) (string, error) {
		return filepath.Join(directory, "npm"), nil
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
