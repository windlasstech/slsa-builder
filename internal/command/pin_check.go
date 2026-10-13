package command

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/windlasstech/slsa-builder/internal/diagnostic"
	"github.com/windlasstech/slsa-builder/internal/toolchainpins"
)

type pinCheckCommand struct{}

// NewPinCheckCommand creates the disk pin-record/specification parity checker.
func NewPinCheckCommand() Command { return pinCheckCommand{} }

func (pinCheckCommand) Name() string { return "pin-check" }

func (command pinCheckCommand) Execute(ctx context.Context, args []string, out io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	flags := flag.NewFlagSet(command.Name(), flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var inputs toolchainpins.DriftInputs
	flags.StringVar(&inputs.PublishPin, "publish-pin", "", "publish npm pin record path")
	flags.StringVar(&inputs.PairRecord, "pair-record", "", "Node.js/npm pair record path")
	flags.StringVar(&inputs.PublishSpec, "publish-spec", "", "publish specification path")
	flags.StringVar(&inputs.BuildSpec, "build-spec", "", "build specification path")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, err := fmt.Fprintln(out, "Usage: slsa-builder-internal pin-check --publish-pin <path> --pair-record <path> --publish-spec <path> --build-spec <path>")
			return err
		}
		return err
	}
	if flags.NArg() != 0 || inputs.PublishPin == "" || inputs.PairRecord == "" || inputs.PublishSpec == "" || inputs.BuildSpec == "" {
		return errors.New("--publish-pin, --pair-record, --publish-spec, and --build-spec are required with no positional arguments")
	}
	if err := toolchainpins.CheckDrift(inputs); err != nil {
		var failure *toolchainpins.DriftError
		if !errors.As(err, &failure) {
			return err
		}
		entry, reportErr := diagnostic.New(failure.DiagnosticID(), "toolchain.pins.drift", RedactSecrets(err.Error()))
		if reportErr != nil {
			return reportErr
		}
		report, reportErr := diagnostic.Build(nil, []diagnostic.Diagnostic{entry}, nil)
		if reportErr != nil {
			return reportErr
		}
		if reportErr := WriteReport(out, report); reportErr != nil {
			return reportErr
		}
		return ErrVerificationFailure
	}
	_, err := fmt.Fprintln(out, "Toolchain pin records match specification tables.")
	return err
}
