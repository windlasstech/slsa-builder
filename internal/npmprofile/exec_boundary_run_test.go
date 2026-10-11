package npmprofile

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestRunCommandBoundaryOutput(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, script, output, want string
		limit                      int
		reject                     bool
	}{
		{"combined trimmed output", "printf '  first'; printf ' second  ' >&2", "first second", "", 0, true},
		{"exact bound", "printf abc", "abc", "", 3, true},
		{"overflow rejected", "printf abcd", "", "command output exceeds 3 bytes", 3, true},
		{"publish overflow truncated", "printf abcd", "abc", "", 3, false},
		{"nonzero exit", "printf rejected >&2; exit 9", "rejected", "pnpm [] failed: exit status 9: rejected", 100, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeScript(t, root, "pnpm", "#!/bin/sh\n"+test.script+"\n")
			got, err := runCommandAtBoundary(t.Context(), root, filepath.Join(root, "pnpm"), []string{}, nil, commandBoundaryOptions{shimDirectory: root, maximumOutput: test.limit, rejectOutputOverflow: test.reject})
			if got != test.output {
				t.Fatalf("command output = %q; want %q", got, test.output)
			}
			if test.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || err.Error() != test.want {
				t.Fatalf("command error = %v; want %q", err, test.want)
			}
			if test.name == "nonzero exit" {
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) || exitErr.ExitCode() != 9 {
					t.Fatalf("command error = %v; want wrapped exit code 9", err)
				}
			}
		})
	}
}

func TestRunCommandBoundaryExpiredContext(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeScript(t, root, "pnpm", "#!/bin/sh\nprintf should-not-run\n")
	ctx, cancel := context.WithDeadline(t.Context(), time.Unix(0, 0))
	defer cancel()
	got, err := runCommandAtBoundary(ctx, root, filepath.Join(root, "pnpm"), []string{}, nil, commandBoundaryOptions{shimDirectory: root})
	if !errors.Is(err, context.DeadlineExceeded) || err.Error() != "command timed out: context deadline exceeded" || got != "" {
		t.Fatalf("expired command = %q, %v; want empty, deadline exceeded", got, err)
	}
}

func TestRunCommandBoundaryMissingExecutable(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	_, err := validateExecutableContainment(filepath.Join(root, "pnpm"), root)
	var boundary *commandBoundaryError
	if !errors.As(err, &boundary) || boundary.DiagnosticID() != idTrustedCoreBoundaryViolation || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing executable = %v; want boundary diagnostic wrapping os.ErrNotExist", err)
	}
}

func TestRunCommandBoundaryPreservesArgumentsAndEnvironment(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeScript(t, root, "pnpm", "#!/bin/sh\nprintf '%s|%s|%s' \"$#\" \"$1\" \"$BOUNDARY_VALUE\"\n")
	argument := "spaces ; $(must-not-execute)"
	got, err := runCommandAtBoundary(t.Context(), root, filepath.Join(root, "pnpm"), []string{"BOUNDARY_VALUE=isolated"}, []string{argument}, commandBoundaryOptions{shimDirectory: root})
	if err != nil || got != "1|"+argument+"|isolated" {
		t.Fatalf("argv/environment = %q, %v; want literal argument and isolated value", got, err)
	}
}
