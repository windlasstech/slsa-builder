// Package command provides the typed dispatcher for the internal executable.
package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
)

// ExitCodeSuccess indicates that the requested operation completed successfully.
const ExitCodeSuccess = 0

// ExitCodeVerificationFailure indicates that a completed policy or verification check failed.
const ExitCodeVerificationFailure = 1

// ExitCodeInvocationFailure indicates that the requested operation could not be executed.
const ExitCodeInvocationFailure = 2

const verifierExecutionFailureID = "windlass.verify.error.verifier-execution-failure"

// ErrVerificationFailure indicates that a command already emitted a completed policy-failure report.
var ErrVerificationFailure = errors.New("verification failure reported")

// ErrInvocationFailure indicates that a command already emitted an invocation-failure report.
var ErrInvocationFailure = errors.New("invocation failure reported")

// Command is an internal subcommand that can be registered with a Dispatcher.
type Command interface {
	Name() string
	Execute(context.Context, []string, io.Writer) error
}

// Result is the process outcome returned by a Dispatcher.
type Result struct {
	ExitCode int
}

// Dispatcher routes an invocation to a registered internal subcommand.
type Dispatcher struct {
	commands map[string]Command
}

// NewDispatcher creates a dispatcher from the commands supplied by the executable.
func NewDispatcher(commands ...Command) *Dispatcher {
	registered := make(map[string]Command, len(commands))
	for _, command := range commands {
		registered[command.Name()] = command
	}

	return &Dispatcher{commands: registered}
}

// Dispatch executes the selected command or reports an invocation failure.
func (d *Dispatcher) Dispatch(ctx context.Context, args []string, out io.Writer) Result {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		if err := writeUsage(out, d.commands); err != nil {
			return Result{ExitCode: ExitCodeInvocationFailure}
		}
		return Result{ExitCode: ExitCodeSuccess}
	}

	selected, ok := d.commands[args[0]]
	if !ok {
		if err := writeInvocationReport(out, fmt.Sprintf("unknown subcommand %q", args[0])); err != nil {
			return Result{ExitCode: ExitCodeInvocationFailure}
		}
		return Result{ExitCode: ExitCodeInvocationFailure}
	}

	if err := selected.Execute(ctx, args[1:], out); err != nil {
		if errors.Is(err, ErrVerificationFailure) {
			return Result{ExitCode: ExitCodeVerificationFailure}
		}
		if errors.Is(err, ErrInvocationFailure) {
			return Result{ExitCode: ExitCodeInvocationFailure}
		}
		if reportErr := writeInvocationReport(out, fmt.Sprintf("subcommand %q failed: %v", selected.Name(), err)); reportErr != nil {
			return Result{ExitCode: ExitCodeInvocationFailure}
		}
		return Result{ExitCode: ExitCodeInvocationFailure}
	}

	return Result{ExitCode: ExitCodeSuccess}
}

func writeUsage(out io.Writer, commands map[string]Command) error {
	if _, err := fmt.Fprintln(out, "Usage: slsa-builder-internal [subcommand] [args...]"); err != nil {
		return err
	}
	if len(commands) == 0 {
		_, err := fmt.Fprintln(out, "No subcommands registered.")
		return err
	}

	if _, err := fmt.Fprintln(out, "Registered subcommands:"); err != nil {
		return err
	}
	names := make([]string, 0, len(commands))
	for name := range commands {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if _, err := fmt.Fprintf(out, "  %s\n", name); err != nil {
			return err
		}
	}
	return nil
}

// Diagnostic is one stable machine-readable report entry.
type Diagnostic struct {
	ID       string `json:"id"`
	Severity string `json:"severity"`
	Category string `json:"category"`
	Check    string `json:"check"`
	Message  string `json:"message"`
}

// Report is the shared machine-readable command result.
type Report struct {
	SchemaVersion string       `json:"schema_version"`
	Result        string       `json:"result"`
	ExitCode      int          `json:"exit_code"`
	PrimaryID     *string      `json:"primary_id"`
	RunInvocation *string      `json:"run_invocation"`
	Diagnostics   []Diagnostic `json:"diagnostics"`
}

// WriteReport emits one JSON report object.
func WriteReport(out io.Writer, report Report) error {
	return json.NewEncoder(out).Encode(report)
}

// writeInvocationReport is the migration seam for internal/diagnostic (C03).
func writeInvocationReport(out io.Writer, message string) error {
	report := Report{
		SchemaVersion: "1",
		Result:        "fail",
		ExitCode:      ExitCodeInvocationFailure,
		PrimaryID:     stringPointer(verifierExecutionFailureID),
		Diagnostics: []Diagnostic{{
			ID:       verifierExecutionFailureID,
			Severity: "error",
			Category: "verifier-execution-failure",
			Check:    "command.dispatch",
			Message:  message,
		}},
	}

	return WriteReport(out, report)
}

func stringPointer(value string) *string {
	return &value
}
