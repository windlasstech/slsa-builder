package command

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/npmprofile"
)

func TestSourceObservationCanonicalValues(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, ref, kind, revision, repository string
		want                                  bool
	}{
		{"branch", "refs/heads/main", "branch", builtRevision, "example/project", true},
		{"tag", "refs/tags/v1.0.0", "tag", builtRevision, "example/project", true},
		{"short ref", "main", "branch", builtRevision, "example/project", false},
		{"newline", "refs/heads/main\n", "branch", builtRevision, "example/project", false},
		{"carriage return", "refs/heads/main\r", "branch", builtRevision, "example/project", false},
		{"nul", "refs/heads/main\x00", "branch", builtRevision, "example/project", false},
		{"unknown type", "refs/heads/main", "pull", builtRevision, "example/project", false},
		{"short revision", "refs/heads/main", "branch", "0123", "example/project", false},
		{"uppercase revision", "refs/heads/main", "branch", strings.ToUpper(builtRevision), "example/project", false},
		{"nonhex revision", "refs/heads/main", "branch", strings.Repeat("z", 40), "example/project", false},
		{"repository URL", "refs/heads/main", "branch", builtRevision, "https://github.com/example/project", false},
		{"repository traversal", "refs/heads/main", "branch", builtRevision, "example/..", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := validSourceObservation(test.ref, test.kind, test.revision, test.repository); got != test.want {
				t.Fatalf("validSourceObservation(%q, %q, %q, %q) = %t, want %t", test.ref, test.kind, test.revision, test.repository, got, test.want)
			}
		})
	}
}

func TestSourceRefOutputFailureDoesNotResolveOrReport(t *testing.T) {
	t.Parallel()
	command := npmProfileSourceRefCommand{resolve: func(context.Context, string, string, string) (string, error) {
		t.Fatal("resolver called without a source override")
		return "", nil
	}}
	var output bytes.Buffer
	err := command.Execute(t.Context(), []string{
		"--ref", "refs/heads/main", "--ref-type", "branch", "--revision", builtRevision,
		"--observed-repository", "example/project", "--github-output", t.TempDir(),
	}, &output)
	if err == nil || !strings.Contains(err.Error(), "regular non-symlink file") || output.Len() != 0 {
		t.Fatalf("Execute() = %v, output = %q, want output-file rejection without report", err, output.String())
	}
}

func TestSourceRefWriterFailurePreservesCause(t *testing.T) {
	t.Parallel()
	cause := errors.New("report sink failed")
	command := NewNPMProfileSourceRefCommand()
	err := command.Execute(t.Context(), []string{
		"--source-ref", "refs/heads/other", "--ref", "refs/heads/main", "--ref-type", "branch",
		"--revision", builtRevision, "--observed-repository", "example/project",
		"--github-output", filepath.Join(t.TempDir(), "output"),
	}, &stagedWriter{err: cause})
	if !errors.Is(err, cause) || errors.Is(err, ErrVerificationFailure) {
		t.Fatalf("Execute() = %v, want writer cause without report sentinel", err)
	}
}

func TestSourceRefFailurePassesThroughOtherErrors(t *testing.T) {
	t.Parallel()
	for _, cause := range []error{errors.New("resolver unavailable"), newIdentityPolicyBindingError(npmprofile.IDReleaseRefMismatch, "different policy error")} {
		t.Run(cause.Error(), func(t *testing.T) {
			t.Parallel()
			if err := writeSourceRefFailure(io.Discard, cause); err != cause {
				t.Fatalf("writeSourceRefFailure() = %v, want unchanged %v", err, cause)
			}
		})
	}
}

func TestSourceRefExecuteRejectsInvalidObservation(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	err := NewNPMProfileSourceRefCommand().Execute(t.Context(), []string{
		"--ref", "main", "--ref-type", "branch", "--revision", builtRevision,
		"--observed-repository", "example/project", "--github-output", filepath.Join(t.TempDir(), "output"),
	}, &output)
	if err == nil || err.Error() != "source observations must be canonical GitHub ref, type, revision, and repository values" || output.Len() != 0 {
		t.Fatalf("Execute() = %v, output = %q, want canonical-observation rejection without report", err, output.String())
	}
}
