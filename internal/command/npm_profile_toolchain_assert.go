package command

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/windlasstech/slsa-builder/internal/diagnostic"
	"github.com/windlasstech/slsa-builder/internal/npmprofile"
)

type npmProfileToolchainAssertCommand struct {
	assert func(context.Context, bool) error
}

// NewNPMProfileToolchainAssertCommand creates the pre-install pair assertion adapter.
func NewNPMProfileToolchainAssertCommand() Command {
	return npmProfileToolchainAssertCommand{assert: npmprofile.AssertToolchainPair}
}

func (npmProfileToolchainAssertCommand) Name() string { return "npm-profile-toolchain-assert" }

func (command npmProfileToolchainAssertCommand) Execute(ctx context.Context, args []string, out io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	flags := flag.NewFlagSet(command.Name(), flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	nodeOnly := flags.Bool("node-only", false, "assert the Node.js runtime without invoking bundled npm")
	reportPath := flags.String("report-path", "", "optional path for persisting a failure report")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, err := fmt.Fprintln(out, "Usage: slsa-builder-internal npm-profile-toolchain-assert [--node-only] [--report-path <path>]")
			return err
		}
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("no positional arguments are accepted")
	}
	if err := command.assert(ctx, *nodeOnly); err != nil {
		var mismatch *npmprofile.ToolchainPairMismatchError
		if !errors.As(err, &mismatch) {
			return err
		}
		entry, reportErr := diagnostic.New(mismatch.DiagnosticID(), "npm.build.toolchain", "executed toolchain differs from the embedded Node.js/npm pair")
		if reportErr != nil {
			return reportErr
		}
		report, reportErr := diagnostic.Build(nil, []diagnostic.Diagnostic{entry}, nil)
		if reportErr != nil {
			return reportErr
		}
		if *reportPath != "" {
			encoded, encodeErr := report.CanonicalJSON()
			if encodeErr != nil {
				return encodeErr
			}
			if writeErr := WriteFileAtomic(*reportPath, encoded, 0o600); writeErr != nil {
				return writeErr
			}
		}
		if err := WriteReport(out, report); err != nil {
			return err
		}
		return ErrVerificationFailure
	}
	_, err := fmt.Fprintln(out, "toolchain pair assertion passed")
	return err
}
