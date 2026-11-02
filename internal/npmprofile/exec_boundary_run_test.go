package npmprofile

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
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

func TestRunCommandBoundaryOperationalLogSpawnFailure(t *testing.T) {
	t.Parallel()
	// Given an accepted executable and a working directory that does not exist.
	root := t.TempDir()
	executable := filepath.Join(root, "pnpm")
	writeScript(t, root, "pnpm", "#!/bin/sh\nprintf should-not-run\n")
	missingDirectory := filepath.Join(root, "missing-directory-canary")
	var log bytes.Buffer
	ctx := WithOperationalDebugOutput(t.Context(), &log)

	// When process creation fails before the child starts.
	got, err := runCommandAtBoundary(ctx, missingDirectory, executable, []string{"BOUNDARY_VALUE=" + operationalEnvironmentCanary}, []string{operationalArgvCanary}, commandBoundaryOptions{shimDirectory: root})

	// Then the original spawn error survives and the completion records only the start failure.
	if got != "" || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("spawn result = %q, %v; want empty output and os.ErrNotExist", got, err)
	}
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) {
		t.Fatalf("spawn error type = %T; want *os.PathError", err)
	}
	fields := requireOperationalLogPair(t, log.String(), "pnpm")
	if fields["started"] != "false" || fields["outcome"] != "start-failed" || fields["exit"] != "" || fields["signal"] != "" {
		t.Fatalf("start-failure fields = %v; want started=false outcome=start-failed without exit or signal", fields)
	}
	if fields["output_truncated"] != "false" {
		t.Fatalf("output_truncated = %q; want false", fields["output_truncated"])
	}
	requireOperationalElapsed(t, fields)
	sensitive := []string{root, executable, missingDirectory, operationalArgvCanary, operationalEnvironmentCanary, err.Error()}
	assertOperationalLogSafe(t, log.String(), sensitive...)
}

func TestRunCommandBoundaryOperationalLogTimeout(t *testing.T) {
	t.Parallel()
	// Given a real blocking child and a short parent deadline.
	root := t.TempDir()
	executable := filepath.Join(root, "pnpm")
	writeScript(t, root, "pnpm", "#!/bin/sh\nprintf '"+operationalChildOutputCanary+"'\nexec /bin/sleep 60\n")
	deadline, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	var log bytes.Buffer
	ctx := WithOperationalDebugOutput(deadline, &log)

	// When the deadline interrupts the started child.
	got, err := runCommandAtBoundary(ctx, root, executable, []string{"BOUNDARY_VALUE=" + operationalEnvironmentCanary}, []string{operationalArgvCanary}, commandBoundaryOptions{shimDirectory: root})

	// Then the original timeout result survives and the completion reports only the observed timeout.
	if got != operationalChildOutputCanary || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout result = %q, %v; want child output and DeadlineExceeded", got, err)
	}
	fields := requireOperationalLogPair(t, log.String(), "pnpm")
	if fields["started"] != "true" || fields["outcome"] != "timeout" || fields["exit"] != "" || fields["signal"] != "" {
		t.Fatalf("timeout fields = %v; want started=true outcome=timeout without exit or signal", fields)
	}
	if fields["output_truncated"] != "false" {
		t.Fatalf("output_truncated = %q; want false", fields["output_truncated"])
	}
	requireOperationalElapsed(t, fields)
	sensitive := []string{root, executable, operationalArgvCanary, operationalEnvironmentCanary, operationalChildOutputCanary, err.Error()}
	assertOperationalLogSafe(t, log.String(), sensitive...)
}

func TestRunCommandBoundaryOperationalLogCancellation(t *testing.T) {
	t.Parallel()
	// Given a real blocking child that signals through a private FIFO after emitting output.
	root := t.TempDir()
	executable := filepath.Join(root, "pnpm")
	writeScript(t, root, "pnpm", "#!/bin/sh\nprintf '"+operationalChildOutputCanary+"'\nprintf started > child-started\nexec /bin/sleep 60\n")
	markerPath := filepath.Join(root, "child-started")
	if err := syscall.Mkfifo(markerPath, 0o600); err != nil {
		t.Fatal(err)
	}
	marker, err := os.OpenFile(markerPath, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer marker.Close()
	commandContext, cancel := context.WithCancel(t.Context())
	defer cancel()
	var log bytes.Buffer
	ctx := WithOperationalDebugOutput(commandContext, &log)
	type result struct {
		output string
		err    error
	}
	resultChannel := make(chan result, 1)
	go func() {
		output, runErr := runCommandAtBoundary(ctx, root, executable, []string{"BOUNDARY_VALUE=" + operationalEnvironmentCanary}, []string{operationalArgvCanary}, commandBoundaryOptions{shimDirectory: root})
		resultChannel <- result{output: output, err: runErr}
	}()

	// When the child-start marker arrives and the parent cancels the command.
	markerRead := make(chan error, 1)
	go func() {
		contents := make([]byte, len("started"))
		_, readErr := io.ReadFull(marker, contents)
		markerRead <- readErr
	}()
	select {
	case readErr := <-markerRead:
		if readErr != nil {
			cancel()
			<-resultChannel
			t.Fatalf("read child-start marker: %v", readErr)
		}
	case <-time.After(5 * time.Second):
		cancel()
		<-resultChannel
		t.Fatal("timed out waiting for child-start marker")
	}
	cancel()
	var got result
	select {
	case got = <-resultChannel:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for canceled command")
	}

	// Then the command result survives and the completion reports cancellation, not a fabricated exit.
	if got.output != operationalChildOutputCanary || got.err == nil {
		t.Fatalf("cancellation result = %q, %v; want child output and a command error", got.output, got.err)
	}
	fields := requireOperationalLogPair(t, log.String(), "pnpm")
	if fields["started"] != "true" || fields["outcome"] != "canceled" || fields["exit"] != "" || fields["signal"] != "" {
		t.Fatalf("cancellation fields = %v; want started=true outcome=canceled without exit or signal", fields)
	}
	if fields["output_truncated"] != "false" {
		t.Fatalf("output_truncated = %q; want false", fields["output_truncated"])
	}
	requireOperationalElapsed(t, fields)
	sensitive := []string{root, executable, markerPath, operationalArgvCanary, operationalEnvironmentCanary, operationalChildOutputCanary, got.err.Error()}
	assertOperationalLogSafe(t, log.String(), sensitive...)
}
