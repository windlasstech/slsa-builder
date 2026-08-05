// Package command provides the typed dispatcher for the internal executable.
package command

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/windlasstech/slsa-builder/internal/diagnostic"
)

// ExitCodeSuccess indicates that the requested operation completed successfully.
const ExitCodeSuccess = 0

// ExitCodeVerificationFailure indicates that a completed policy or verification check failed.
const ExitCodeVerificationFailure = 1

// ExitCodeInvocationFailure indicates that the requested operation could not be executed.
const ExitCodeInvocationFailure = 2

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
		message := RedactSecrets(fmt.Sprintf("subcommand %q failed: %v", selected.Name(), err))
		if reportErr := writeInvocationReport(out, message); reportErr != nil {
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

// Diagnostic is the shared closed diagnostic entry.
type Diagnostic = diagnostic.Diagnostic

// Report is the shared closed machine-readable command result.
type Report = diagnostic.Report

// WriteReport emits one JSON report object.
func WriteReport(out io.Writer, report Report) error {
	encoded, err := report.CanonicalJSON()
	if err != nil {
		return err
	}
	if _, err := out.Write(encoded); err != nil {
		return err
	}
	_, err = fmt.Fprintln(out)
	return err
}

func writeInvocationReport(out io.Writer, message string) error {
	entry, err := diagnostic.New(diagnostic.IDVerifierExecutionFailure, "command.dispatch", RedactSecrets(message))
	if err != nil {
		return err
	}
	report, err := diagnostic.Build(nil, []diagnostic.Diagnostic{entry}, nil)
	if err != nil {
		return err
	}
	return WriteReport(out, report)
}
