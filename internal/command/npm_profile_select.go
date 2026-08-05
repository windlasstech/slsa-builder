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

type npmProfileSelectCommand struct{}

// NewNPMProfileSelectCommand creates the npm package and manager policy subcommand.
func NewNPMProfileSelectCommand() Command {
	return npmProfileSelectCommand{}
}

func (npmProfileSelectCommand) Name() string {
	return "npm-profile-select"
}

func (npmProfileSelectCommand) Execute(ctx context.Context, args []string, out io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	flags := flag.NewFlagSet("npm-profile-select", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	repositoryRoot := flags.String("repository-root", "", "checked-out repository root")
	packageDirectory := flags.String("package-directory", "", "repository-relative package directory")
	observedRepository := flags.String("observed-repository", "", "observed caller repository")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, writeErr := fmt.Fprintln(out, "Usage: slsa-builder-internal npm-profile-select --repository-root <path> --package-directory <path> --observed-repository <owner/repository>")
			return writeErr
		}
		return err
	}
	if *repositoryRoot == "" || *packageDirectory == "" || *observedRepository == "" {
		return errors.New("--repository-root, --package-directory, and --observed-repository are required")
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", flags.Args())
	}

	result, err := npmprofile.Analyze(npmprofile.Config{
		RepositoryRoot:     *repositoryRoot,
		PackageDirectory:   *packageDirectory,
		ObservedRepository: *observedRepository,
	})
	if err != nil {
		return err
	}
	if err := WriteReport(out, result.Report); err != nil {
		return err
	}
	if result.Report.ExitCode == diagnostic.ExitCodePolicyFailure {
		return ErrVerificationFailure
	}
	return nil
}
