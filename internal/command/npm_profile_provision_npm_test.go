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
