package npmprofile

import (
	"bytes"
	"errors"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

const (
	operationalLogPrefix         = "windlass operational: "
	operationalArgvCanary        = "argv-canary-7f3c2a"
	operationalEnvironmentCanary = "environment-canary-91bd4e"
	operationalChildOutputCanary = "child-output-canary-65af08"
	operationalOutputScript      = `printf '%s|%s|%s' 'child-output-canary-65af08' "$1" "$BOUNDARY_VALUE"`
)

func TestRunCommandBoundaryOperationalLogCompletion(t *testing.T) {
	t.Parallel()
	fullOutput := operationalChildOutputCanary + "|" + operationalArgvCanary + "|" + operationalEnvironmentCanary
	for _, test := range []struct {
		name, script, outcome, exit, output string
		maximumOutput                       int
		truncated, signaled, wantFailure    bool
	}{
		{name: "exit zero", script: operationalOutputScript, outcome: "exit", exit: "0", output: fullOutput},
		{name: "nonzero exit", script: operationalOutputScript + "; exit 9", outcome: "exit", exit: "9", output: fullOutput, wantFailure: true},
		{name: "signal", script: operationalOutputScript + "; kill -TERM $$", outcome: "signal", output: fullOutput, signaled: true, wantFailure: true},
		{name: "duration and truncation", script: operationalOutputScript, outcome: "exit", exit: "0", output: "child-ou", maximumOutput: 8, truncated: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			// Given an accepted executable whose arguments, environment, and output contain distinct canaries.
			root := t.TempDir()
			executable := filepath.Join(root, "pnpm")
			writeScript(t, root, "pnpm", "#!/bin/sh\n"+test.script+"\n")
			environment := []string{"BOUNDARY_VALUE=" + operationalEnvironmentCanary}
			arguments := []string{operationalArgvCanary}
			options := commandBoundaryOptions{shimDirectory: root, maximumOutput: test.maximumOutput}
			wantOutput, wantErr := runCommandAtBoundary(t.Context(), root, executable, environment, arguments, options)
			if wantOutput != test.output || (wantErr != nil) != test.wantFailure {
				t.Fatalf("baseline result = %q, %v; want %q, failure=%t", wantOutput, wantErr, test.output, test.wantFailure)
			}
			var log bytes.Buffer

			// When the same command runs with human-readable diagnostics enabled.
			ctx := WithOperationalDebugOutput(t.Context(), &log)
			got, err := runCommandAtBoundary(ctx, root, executable, environment, arguments, options)

			// Then diagnostics describe completion without changing the command result or disclosing raw values.
			if got != wantOutput || (err == nil) != (wantErr == nil) {
				t.Fatalf("command result = %q, %v; want unchanged %q, %v", got, err, wantOutput, wantErr)
			}
			if err != nil && err.Error() != wantErr.Error() {
				t.Fatalf("command error = %q; want unchanged %q", err, wantErr)
			}
			fields := requireOperationalLogPair(t, log.String(), "pnpm")
			if fields["started"] != "true" || fields["outcome"] != test.outcome {
				t.Fatalf("completion fields = %v; want started=true outcome=%s", fields, test.outcome)
			}
			if test.signaled {
				if fields["signal"] == "" || fields["exit"] != "" {
					t.Fatalf("signal fields = %v; want a signal and no exit code", fields)
				}
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) {
					t.Fatalf("command error = %v; want preserved exit error", err)
				}
				status, ok := exitErr.Sys().(syscall.WaitStatus)
				if !ok || status.Signal() != syscall.SIGTERM {
					t.Fatalf("command error = %v; want preserved SIGTERM exit error", err)
				}
			} else if fields["exit"] != test.exit || fields["signal"] != "" {
				t.Fatalf("exit fields = %v; want exit=%s and no signal", fields, test.exit)
			}
			if fields["output_truncated"] != strconv.FormatBool(test.truncated) {
				t.Fatalf("output_truncated = %q; want %t", fields["output_truncated"], test.truncated)
			}
			requireOperationalElapsed(t, fields)
			sensitive := []string{root, executable, operationalArgvCanary, operationalEnvironmentCanary, operationalChildOutputCanary}
			if err != nil {
				sensitive = append(sensitive, err.Error())
			}
			assertOperationalLogSafe(t, log.String(), sensitive...)
		})
	}
}

func TestRunCommandBoundaryOperationalLogWriterFailure(t *testing.T) {
	t.Parallel()
	// Given a command with a characterized result and a writer that rejects every diagnostic write.
	root := t.TempDir()
	writeScript(t, root, "pnpm", "#!/bin/sh\n"+operationalOutputScript+"; exit 9\n")
	writer := &failingOperationalLogWriter{}
	ctx := WithOperationalDebugOutput(t.Context(), writer)

	// When the command runs with the failing diagnostic writer.
	got, err := runCommandAtBoundary(ctx, root, filepath.Join(root, "pnpm"), []string{"BOUNDARY_VALUE=" + operationalEnvironmentCanary}, []string{operationalArgvCanary}, commandBoundaryOptions{shimDirectory: root})

	// Then only the command's original output and error are returned.
	wantOutput := operationalChildOutputCanary + "|" + operationalArgvCanary + "|" + operationalEnvironmentCanary
	wantError := "pnpm [" + operationalArgvCanary + "] failed: exit status 9: " + wantOutput
	if got != wantOutput || err == nil || err.Error() != wantError {
		t.Fatalf("command result = %q, %v; want %q, %q", got, err, wantOutput, wantError)
	}
	if errors.Is(err, errOperationalLogWriter) {
		t.Fatalf("diagnostic writer error escaped: %v", err)
	}
	if writer.calls == 0 {
		t.Fatal("diagnostic writer was not called")
	}
}

var errOperationalLogWriter = errors.New("operational log writer failure")

type failingOperationalLogWriter struct{ calls int }

func (writer *failingOperationalLogWriter) Write([]byte) (int, error) {
	writer.calls++
	return 0, errOperationalLogWriter
}

type operationalLogRecord struct {
	start  bool
	fields map[string]string
}

func requireOperationalLogPair(t *testing.T, raw, operation string) map[string]string {
	t.Helper()
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		t.Fatal("operational log is empty")
	}
	lines := strings.Split(trimmed, "\n")
	if len(lines) != 2 {
		t.Fatalf("operational log lines = %d; want start and completion records: %q", len(lines), raw)
	}
	records := make([]operationalLogRecord, 0, len(lines))
	for _, line := range lines {
		if !strings.HasPrefix(line, operationalLogPrefix) {
			t.Fatalf("operational line lacks the human prefix: %q", line)
		}
		record := operationalLogRecord{fields: map[string]string{}}
		for _, token := range strings.Fields(strings.TrimPrefix(line, operationalLogPrefix)) {
			if token == "start" {
				record.start = true
				continue
			}
			key, value, found := strings.Cut(token, "=")
			if !found || key == "" || value == "" {
				t.Fatalf("malformed operational field %q in %q", token, line)
			}
			if _, duplicate := record.fields[key]; duplicate {
				t.Fatalf("duplicate operational field %q in %q", key, line)
			}
			record.fields[key] = value
		}
		if record.fields["operation"] != operation {
			t.Fatalf("operation = %q; want fixed label %q", record.fields["operation"], operation)
		}
		records = append(records, record)
	}
	if !records[0].start || len(records[0].fields) != 1 || records[1].start {
		t.Fatalf("operational records = %v; want one start notice followed by one completion", records)
	}
	return records[1].fields
}

func requireOperationalElapsed(t *testing.T, fields map[string]string) {
	t.Helper()
	elapsed, err := time.ParseDuration(fields["elapsed"])
	if err != nil || elapsed < 0 {
		t.Fatalf("elapsed = %q, %v; want a nonnegative duration", fields["elapsed"], err)
	}
}

func assertOperationalLogSafe(t *testing.T, raw string, sensitive ...string) {
	t.Helper()
	for _, value := range sensitive {
		if value != "" && strings.Contains(raw, value) {
			t.Fatalf("operational log disclosed %q: %q", value, raw)
		}
	}
	if strings.ContainsAny(raw, "{}") {
		t.Fatalf("operational log contains JSON delimiters: %q", raw)
	}
}
