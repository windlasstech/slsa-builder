package command

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/windlasstech/slsa-builder/internal/workflowcheck"
)

type workflowCheckCommand struct{}

// NewWorkflowCheckCommand creates the trusted workflow conformance subcommand.
func NewWorkflowCheckCommand() Command { return workflowCheckCommand{} }

func (workflowCheckCommand) Name() string { return "workflow-check" }

func (workflowCheckCommand) Execute(ctx context.Context, args []string, out io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	flags := flag.NewFlagSet("workflow-check", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	workflowPath := flags.String("workflow", "", "workflow file path")
	jobName := flags.String("job", "", "job to validate")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, writeErr := fmt.Fprintln(out, "Usage: slsa-builder-internal workflow-check --workflow <path> --job <build|provenance-sign>")
			return writeErr
		}
		return err
	}
	if *workflowPath == "" || *jobName == "" || flags.NArg() != 0 {
		return errors.New("--workflow and --job are required with no positional arguments")
	}
	var (
		result any
		err    error
	)
	switch *jobName {
	case "build":
		result, err = workflowcheck.CheckBuildJob(*workflowPath)
	case "provenance-sign":
		result, err = workflowcheck.CheckProvenanceSignJob(*workflowPath)
	default:
		return fmt.Errorf("unsupported workflow job %q", *jobName)
	}
	if err != nil {
		return err
	}
	if err := json.NewEncoder(out).Encode(result); err != nil {
		return fmt.Errorf("encode workflow-check result: %w", err)
	}
	return nil
}
