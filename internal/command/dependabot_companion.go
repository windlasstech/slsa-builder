package command

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/windlasstech/slsa-builder/internal/dependabot"
)

type dependabotCompanionCommand struct{}

func NewDependabotCompanionCommand() Command    { return dependabotCompanionCommand{} }
func (dependabotCompanionCommand) Name() string { return "dependabot-companion" }

func (command dependabotCompanionCommand) Execute(ctx context.Context, args []string, out io.Writer) error {
	flags := flag.NewFlagSet(command.Name(), flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var inputs dependabot.Inputs
	var mode string
	flags.StringVar(&mode, "mode", "", "dry-run, plan, or apply")
	flags.StringVar(&inputs.EventPath, "event", "", "local projection for dry-run; workflow_run payload for plan/apply")
	flags.StringVar(&inputs.Snapshot, "snapshot", "", "local snapshot directory")
	flags.StringVar(&inputs.TrustedRoot, "trusted-root", "", "trusted checkout")
	flags.StringVar(&inputs.TrustedSHA, "trusted-sha", "", "trusted checkout commit")
	flags.StringVar(&inputs.Output, "output", "", "separate plan output directory")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, err := fmt.Fprintln(out, "Usage: slsa-builder-internal dependabot-companion --mode <dry-run|plan|apply> --event <path> --trusted-root <dir> --trusted-sha <40hex> --output <dir> [--snapshot <dir> (dry-run only)]")
			return err
		}
		return err
	}
	if flags.NArg() != 0 || inputs.EventPath == "" || inputs.TrustedRoot == "" || inputs.TrustedSHA == "" || inputs.Output == "" {
		return errors.New("mode, event, trusted-root, trusted-sha and output are required; positional arguments are forbidden")
	}
	plan, err := executeCompanion(ctx, mode, inputs)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "%s: %s\n", plan.Status, plan.Reason)
	return err
}

func executeCompanion(ctx context.Context, mode string, inputs dependabot.Inputs) (dependabot.Plan, error) {
	switch mode {
	case "dry-run":
		if inputs.Snapshot == "" {
			return dependabot.Plan{}, dependabot.ErrInput
		}
		return (dependabot.Planner{}).DryRun(ctx, inputs)
	case "plan", "apply":
		if inputs.Snapshot != "" {
			return dependabot.Plan{}, dependabot.ErrInput
		}
		remote, err := dependabot.NewRemote(os.Getenv("GITHUB_TOKEN"))
		if err != nil {
			return dependabot.Plan{}, err
		}
		defer remote.Close()
		if mode == "apply" {
			return remote.Apply(ctx, inputs, os.Getenv("APP_SLUG"))
		}
		return remote.Plan(ctx, inputs)
	default:
		return dependabot.Plan{}, dependabot.ErrInput
	}
}
