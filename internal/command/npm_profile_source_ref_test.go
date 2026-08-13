package command

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNPMProfileSourceRefCommand(t *testing.T) {
	t.Parallel()

	dispatch := func(t *testing.T, arguments ...string) (string, int, string) {
		t.Helper()
		outputFile := filepath.Join(t.TempDir(), "github-output")
		arguments = append(arguments, "--github-output", outputFile)
		var output bytes.Buffer
		result := NewDispatcher(NewNPMProfileSourceRefCommand()).Dispatch(context.Background(), append([]string{"npm-profile-source-ref"}, arguments...), &output)
		contents, err := os.ReadFile(outputFile)
		if err != nil && !os.IsNotExist(err) {
			t.Fatalf("read github-output: %v", err)
		}
		return output.String(), result.ExitCode, string(contents)
	}

	t.Run("omitted passes the invocation identity through", func(t *testing.T) {
		t.Parallel()
		report, exitCode, outputs := dispatch(t,
			"--ref", "refs/tags/v1.2.3", "--ref-type", "tag",
			"--revision", "0123456789abcdef0123456789abcdef01234567",
			"--observed-repository", "example/project")
		if exitCode != 0 {
			t.Fatalf("exit code = %d, report = %s", exitCode, report)
		}
		if !strings.Contains(outputs, "built-ref=refs/tags/v1.2.3\n") ||
			!strings.Contains(outputs, "built-revision=0123456789abcdef0123456789abcdef01234567\n") {
			t.Fatalf("outputs = %q", outputs)
		}
	})

	t.Run("invalid form fails with the registered diagnostic", func(t *testing.T) {
		t.Parallel()
		report, exitCode, _ := dispatch(t,
			"--source-ref", "refs/heads/main",
			"--ref", "refs/heads/main", "--ref-type", "branch",
			"--revision", "0123456789abcdef0123456789abcdef01234567",
			"--observed-repository", "example/project")
		assertPrimaryDiagnostic(t, []byte(report), exitCode, 1, "windlass.verify.error.source-ref-invalid")
	})

	t.Run("invocation tag conflict fails with the registered diagnostic", func(t *testing.T) {
		t.Parallel()
		report, exitCode, _ := dispatch(t,
			"--source-ref", "refs/tags/v1.2.4",
			"--ref", "refs/tags/v1.2.3", "--ref-type", "tag",
			"--revision", "0123456789abcdef0123456789abcdef01234567",
			"--observed-repository", "example/project")
		assertPrimaryDiagnostic(t, []byte(report), exitCode, 1, "windlass.verify.error.source-ref-invalid")
	})

	t.Run("missing observed repository coordinates", func(t *testing.T) {
		t.Parallel()
		var output bytes.Buffer
		result := NewDispatcher(NewNPMProfileSourceRefCommand()).Dispatch(context.Background(),
			[]string{"npm-profile-source-ref", "--ref", "refs/tags/v1.2.3"}, &output)
		if result.ExitCode != 2 {
			t.Fatalf("exit code = %d, want invocation failure", result.ExitCode)
		}
	})
}
