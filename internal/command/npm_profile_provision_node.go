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

var nodeProvisionOutputs = NewOutputAllowlist("node-bin")

type npmProfileProvisionNodeCommand struct {
	provision func(context.Context, string) (string, error)
}

// NewNPMProfileProvisionNodeCommand creates the digest-verified Node.js runtime provisioner.
func NewNPMProfileProvisionNodeCommand() Command {
	return npmProfileProvisionNodeCommand{provision: npmprofile.ProvisionNode}
}

func (npmProfileProvisionNodeCommand) Name() string { return "npm-profile-provision-node" }

func (command npmProfileProvisionNodeCommand) Execute(ctx context.Context, args []string, out io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	flags := flag.NewFlagSet(command.Name(), flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	outputDirectory := flags.String("output-directory", "", "parent directory for private Node.js provisioning")
	githubOutput := flags.String("github-output", "", "optional GitHub Actions output file")
	reportPath := flags.String("report-path", "", "optional path for persisting a failure report")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, err := fmt.Fprintln(out, "Usage: slsa-builder-internal npm-profile-provision-node --output-directory <dir> [--github-output <path>] [--report-path <path>]")
			return err
		}
		return err
	}
	if flags.NArg() != 0 || *outputDirectory == "" {
		return errors.New("--output-directory is required with no positional arguments")
	}
	binDir, err := command.provision(ctx, *outputDirectory)
	if err != nil {
		var failure *npmprofile.NodeProvisionError
		if !errors.As(err, &failure) {
			return err
		}
		entry, reportErr := diagnostic.New(failure.DiagnosticID(), "npm.toolchain.provision", failure.Error())
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
		if err := WriteGitHubOutputs(*githubOutput, nodeProvisionOutputs, map[string]string{"node-bin": binDir}); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintln(out, binDir)
	return err
}
