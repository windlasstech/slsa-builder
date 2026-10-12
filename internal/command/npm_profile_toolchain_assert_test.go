package command

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/npmprofile"
)

func TestToolchainAssertCommand_whenNodeOnlySelected(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name     string
		args     []string
		nodeOnly bool
	}{
		{name: "full mode"},
		{name: "node-only mode", args: []string{"--node-only"}, nodeOnly: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			// Given a domain assertion that records the selected mode.
			called := false
			adapter := npmProfileToolchainAssertCommand{assert: func(ctx context.Context, nodeOnly bool) error {
				called = true
				if ctx.Err() != nil || nodeOnly != test.nodeOnly {
					t.Errorf("context = %v, nodeOnly = %v, want %v", ctx.Err(), nodeOnly, test.nodeOnly)
				}
				return nil
			}}
			var out bytes.Buffer
			// When dispatched with the requested mode.
			result := NewDispatcher(adapter).Dispatch(t.Context(), append([]string{adapter.Name()}, test.args...), &out)
			// Then the assertion ran and a short confirmation was emitted.
			if !called || result.ExitCode != 0 || !strings.Contains(out.String(), "toolchain") {
				t.Fatalf("called = %v, exit = %d, output = %q", called, result.ExitCode, out.String())
			}
		})
	}
}

func TestToolchainAssertCommand_whenPairMismatch(t *testing.T) {
	t.Parallel()
	// Given a domain pair mismatch.
	adapter := npmProfileToolchainAssertCommand{assert: func(context.Context, bool) error {
		return &npmprofile.ToolchainPairMismatchError{}
	}}
	var out bytes.Buffer
	// When the command is dispatched.
	result := NewDispatcher(adapter).Dispatch(t.Context(), []string{adapter.Name()}, &out)
	// Then the first ordered error is the toolchain check and exit is 1.
	if result.ExitCode != 1 {
		t.Fatalf("exit = %d, output = %s", result.ExitCode, out.Bytes())
	}
	assertPrimaryDiagnostic(t, out.Bytes(), 1, "windlass.verify.error.toolchain-pair-mismatch")
	var report Report
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Diagnostics) != 1 || report.Diagnostics[0].Check != "npm.build.toolchain" {
		t.Fatalf("diagnostics = %#v, want npm.build.toolchain", report.Diagnostics)
	}
}

func TestToolchainAssertCommand_whenUnexpectedFailure(t *testing.T) {
	t.Parallel()
	// Given a domain failure that is not a classified pair mismatch.
	adapter := npmProfileToolchainAssertCommand{assert: func(context.Context, bool) error {
		return errors.New("node executable vanished")
	}}
	var out bytes.Buffer
	// When the command is dispatched.
	result := NewDispatcher(adapter).Dispatch(t.Context(), []string{adapter.Name()}, &out)
	// Then the unclassified cause becomes a canonical invocation report with exit 2.
	if result.ExitCode != 2 {
		t.Fatalf("exit = %d, output = %s", result.ExitCode, out.Bytes())
	}
	assertPrimaryDiagnostic(t, out.Bytes(), 2, "windlass.verify.error.verifier-execution-failure")
}

func TestToolchainAssertCommand_whenReportWriteFails(t *testing.T) {
	t.Parallel()
	// Given a pair mismatch and a sink that rejects the report.
	adapter := npmProfileToolchainAssertCommand{assert: func(context.Context, bool) error {
		return &npmprofile.ToolchainPairMismatchError{}
	}}
	cause := errors.New("report sink failed")
	// When the verification report cannot be written.
	err := adapter.Execute(t.Context(), nil, &stagedWriter{err: cause})
	// Then the sink cause surfaces instead of the verification-failure sentinel.
	if !errors.Is(err, cause) || errors.Is(err, ErrVerificationFailure) {
		t.Fatalf("Execute() = %v, want sink cause without report sentinel", err)
	}
}

func TestToolchainAssertCommand_whenInvocationInvalid(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"--unknown"}, {"positional"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			t.Parallel()
			// Given invalid command arguments.
			adapter := npmProfileToolchainAssertCommand{assert: func(context.Context, bool) error {
				t.Error("assertion executed for invalid invocation")
				return errors.New("unreachable")
			}}
			var out bytes.Buffer
			// When dispatched.
			result := NewDispatcher(adapter).Dispatch(t.Context(), append([]string{adapter.Name()}, args...), &out)
			// Then dispatch emits the canonical invocation report with exit 2.
			if result.ExitCode != 2 {
				t.Fatalf("exit = %d, output = %s", result.ExitCode, out.Bytes())
			}
			assertPrimaryDiagnostic(t, out.Bytes(), 2, "windlass.verify.error.verifier-execution-failure")
		})
	}
}

func TestToolchainAssertCommand_whenReportPathRequested(t *testing.T) {
	t.Parallel()
	// Given a domain pair mismatch and a requested report path.
	directory := t.TempDir()
	reportPath := filepath.Join(directory, "outcome.json")
	adapter := npmProfileToolchainAssertCommand{assert: func(context.Context, bool) error {
		return &npmprofile.ToolchainPairMismatchError{}
	}}
	var out bytes.Buffer
	// When the mismatch is dispatched with --report-path.
	result := NewDispatcher(adapter).Dispatch(t.Context(), []string{adapter.Name(), "--report-path", reportPath}, &out)
	// Then the specific failure report is also persisted at the path.
	if result.ExitCode != 1 {
		t.Fatalf("exit = %d, output = %s", result.ExitCode, out.Bytes())
	}
	persisted, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("persisted report: %v", err)
	}
	assertPrimaryDiagnostic(t, persisted, 1, "windlass.verify.error.toolchain-pair-mismatch")
}

func TestToolchainAssertCommand_whenReportPathRequestedOnSuccess(t *testing.T) {
	t.Parallel()
	// Given a successful assertion and a requested report path.
	directory := t.TempDir()
	reportPath := filepath.Join(directory, "outcome.json")
	adapter := npmProfileToolchainAssertCommand{assert: func(context.Context, bool) error {
		return nil
	}}
	var out bytes.Buffer
	// When dispatched with --report-path.
	result := NewDispatcher(adapter).Dispatch(t.Context(), []string{adapter.Name(), "--report-path", reportPath}, &out)
	// Then success persists no report file.
	if result.ExitCode != 0 {
		t.Fatalf("exit = %d, output = %s", result.ExitCode, out.Bytes())
	}
	if _, err := os.Stat(reportPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("report path exists after success: %v", err)
	}
}
