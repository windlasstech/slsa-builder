package command

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/windlasstech/slsa-builder/internal/diagnostic"
	"github.com/windlasstech/slsa-builder/internal/fixture"
)

const diagnosticsContractInvalidID = "windlass.verify.error.diagnostics-contract-invalid"
const inputUnavailableID = "windlass.verify.error.input-unavailable"

type fixtureCheckCommand struct{}

// NewFixtureCheckCommand creates the fixture manifest validation subcommand.
func NewFixtureCheckCommand() Command {
	return fixtureCheckCommand{}
}

func (fixtureCheckCommand) Name() string {
	return "fixture-check"
}

func (fixtureCheckCommand) Execute(ctx context.Context, args []string, out io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	flags := flag.NewFlagSet("fixture-check", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	indexPath := flags.String("index", "", "fixture index path")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, writeErr := fmt.Fprintln(out, "Usage: slsa-builder-internal fixture-check --index <path>")
			return writeErr
		}
		return err
	}
	if *indexPath == "" {
		return fmt.Errorf("--index is required")
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", flags.Args())
	}

	_, err := fixture.Load(*indexPath)
	if err == nil {
		return writeDiagnostics(out, nil, nil)
	}
	if errors.Is(err, fixture.ErrInputUnavailable) {
		if reportErr := writeDiagnosticError(out, inputUnavailableID, "fixture.index", err.Error()); reportErr != nil {
			return reportErr
		}
		return ErrInvocationFailure
	}

	if reportErr := writeDiagnosticError(out, diagnosticsContractInvalidID, "fixture.index", err.Error()); reportErr != nil {
		return reportErr
	}
	return ErrVerificationFailure
}

func writeDiagnosticError(out io.Writer, id, check, message string) error {
	entry, err := diagnostic.New(id, check, RedactSecrets(message))
	if err != nil {
		return err
	}
	return writeDiagnostics(out, nil, []diagnostic.Diagnostic{entry})
}

func writeDiagnostics(out io.Writer, runInvocation *string, entries []diagnostic.Diagnostic) error {
	if entries == nil {
		entries = []diagnostic.Diagnostic{}
	}
	report, err := diagnostic.Build(runInvocation, entries, nil)
	if err != nil {
		return err
	}
	return WriteReport(out, report)
}
