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

var npmProvisionOutputs = NewOutputAllowlist("npm-executable")

type npmProfileProvisionNPMCommand struct {
	provision func(context.Context, string) (string, error)
}

// NewNPMProfileProvisionNPMCommand creates the digest-verified publish npm provisioner.
func NewNPMProfileProvisionNPMCommand() Command {
	return npmProfileProvisionNPMCommand{provision: npmprofile.ProvisionNPM}
}

func (npmProfileProvisionNPMCommand) Name() string { return "npm-profile-provision-npm" }

func (command npmProfileProvisionNPMCommand) Execute(ctx context.Context, args []string, out io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	flags := flag.NewFlagSet(command.Name(), flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	outputDirectory := flags.String("output-directory", "", "parent directory for private npm provisioning")
	githubOutput := flags.String("github-output", "", "optional GitHub Actions output file")
	reportPath := flags.String("report-path", "", "optional path for persisting a failure report")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, err := fmt.Fprintln(out, "Usage: slsa-builder-internal npm-profile-provision-npm --output-directory <dir> [--github-output <path>] [--report-path <path>]")
			return err
		}
		return err
	}
	if flags.NArg() != 0 || *outputDirectory == "" {
		return errors.New("--output-directory is required with no positional arguments")
	}
	launcher, err := command.provision(ctx, *outputDirectory)
	if err != nil {
		var failure *npmprofile.NPMProvisionError
		if !errors.As(err, &failure) {
			return err
		}
		entry, reportErr := diagnostic.New(failure.DiagnosticID(), "npm.publish.provision", failure.Error())
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
		if reportErr := WriteReport(out, report); reportErr != nil {
			return reportErr
		}
		return ErrVerificationFailure
	}
	if *githubOutput != "" {
		if err := WriteGitHubOutputs(*githubOutput, npmProvisionOutputs, map[string]string{"npm-executable": launcher}); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintln(out, launcher)
	return err
}
