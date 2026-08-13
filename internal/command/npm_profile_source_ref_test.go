package command

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/diagnostic"
	"github.com/windlasstech/slsa-builder/internal/npmprofile"
)

func TestNPMProfileSourceRefCommand(t *testing.T) {
	t.Parallel()
	const revision = "0123456789abcdef0123456789abcdef01234567"

	t.Run("omitted source ref avoids resolution", func(t *testing.T) {
		output := filepath.Join(t.TempDir(), "github-output")
		command := newNPMProfileSourceRefCommand(func(context.Context, string, string, string) (string, error) {
			t.Fatal("resolver called for omitted source-ref")
			return "", nil
		})
		result, report := dispatchSourceRef(t, command, output,
			"--ref", "refs/tags/v1.2.3", "--ref-type", "tag", "--revision", revision,
			"--observed-repository", "windlasstech/slsa-builder")
		if result.ExitCode != ExitCodeSuccess || report != "" {
			t.Fatalf("result = %#v, report = %s", result, report)
		}
		assertSourceRefOutputs(t, output, "refs/tags/v1.2.3", revision)
	})

	t.Run("ASCII whitespace source ref is byte-identical to omitted", func(t *testing.T) {
		output := filepath.Join(t.TempDir(), "github-output")
		command := newNPMProfileSourceRefCommand(func(context.Context, string, string, string) (string, error) {
			t.Fatal("resolver called for ASCII-whitespace-only source-ref")
			return "", nil
		})
		result, report := dispatchSourceRef(t, command, output,
			"--source-ref", " \t\n\r\v\f", "--ref", "refs/tags/v1.2.3", "--ref-type", "tag", "--revision", revision,
			"--observed-repository", "windlasstech/slsa-builder")
		if result.ExitCode != ExitCodeSuccess || report != "" {
			t.Fatalf("result = %#v, report = %s", result, report)
		}
		assertSourceRefOutputs(t, output, "refs/tags/v1.2.3", revision)
	})

	t.Run("supplied tag resolves", func(t *testing.T) {
		output := filepath.Join(t.TempDir(), "github-output")
		command := newNPMProfileSourceRefCommand(func(ctx context.Context, sourceRef, invocationRef, remote string) (string, error) {
			if ctx.Err() != nil || sourceRef != "refs/tags/v1.2.3" || invocationRef != "refs/heads/main" || remote != "https://github.com/windlasstech/slsa-builder.git" {
				t.Fatalf("resolver arguments = %q, %q, %q", sourceRef, invocationRef, remote)
			}
			return revision, nil
		})
		result, report := dispatchSourceRef(t, command, output,
			"--source-ref", "refs/tags/v1.2.3", "--ref", "refs/heads/main", "--ref-type", "branch",
			"--revision", revision, "--observed-repository", "windlasstech/slsa-builder")
		if result.ExitCode != ExitCodeSuccess || report != "" {
			t.Fatalf("result = %#v, report = %s", result, report)
		}
		assertSourceRefOutputs(t, output, "refs/tags/v1.2.3", revision)
	})

	t.Run("missing tag is policy failure", func(t *testing.T) {
		output := filepath.Join(t.TempDir(), "github-output")
		command := newNPMProfileSourceRefCommand(func(context.Context, string, string, string) (string, error) {
			return "", sourceRefTestError{}
		})
		result, encoded := dispatchSourceRef(t, command, output,
			"--source-ref", "refs/tags/v9.9.9", "--ref", "refs/heads/main", "--ref-type", "branch",
			"--revision", revision, "--observed-repository", "windlasstech/slsa-builder")
		if result.ExitCode != ExitCodeVerificationFailure {
			t.Fatalf("exit code = %d, report = %s", result.ExitCode, encoded)
		}
		var report diagnostic.Report
		if err := json.Unmarshal([]byte(encoded), &report); err != nil {
			t.Fatal(err)
		}
		if report.PrimaryID == nil || *report.PrimaryID != npmprofile.IDSourceRefInvalid {
			t.Fatalf("primary ID = %v", report.PrimaryID)
		}
	})

	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "positional argument", args: []string{"--ref", "refs/heads/main", "--ref-type", "branch", "--revision", revision, "--observed-repository", "windlasstech/slsa-builder", "extra"}},
		{name: "missing observations", args: []string{"--ref", "refs/heads/main"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, _ := dispatchSourceRef(t, NewNPMProfileSourceRefCommand(), filepath.Join(t.TempDir(), "github-output"), test.args...)
			if result.ExitCode != ExitCodeInvocationFailure {
				t.Fatalf("exit code = %d, want %d", result.ExitCode, ExitCodeInvocationFailure)
			}
		})
	}
}

type sourceRefTestError struct{}

func (sourceRefTestError) Error() string { return "tag is not resolvable to a commit" }

func (sourceRefTestError) DiagnosticID() string { return npmprofile.IDSourceRefInvalid }

func dispatchSourceRef(t *testing.T, command Command, output string, args ...string) (Result, string) {
	t.Helper()
	var report bytes.Buffer
	args = append([]string{"npm-profile-source-ref", "--github-output", output}, args...)
	result := NewDispatcher(command).Dispatch(context.Background(), args, &report)
	return result, report.String()
}

func assertSourceRefOutputs(t *testing.T, path, wantRef, wantRevision string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "built-ref=" + wantRef + "\nbuilt-revision=" + wantRevision + "\n"
	if string(data) != want {
		t.Fatalf("outputs = %q, want %q", data, want)
	}
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		if strings.ContainsAny(line, "\r\x00") {
			t.Fatalf("output is not single-line: %q", line)
		}
	}
}
