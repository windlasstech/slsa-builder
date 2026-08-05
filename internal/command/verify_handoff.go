package command

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/windlasstech/slsa-builder/internal/diagnostic"
	"github.com/windlasstech/slsa-builder/internal/digest"
	"github.com/windlasstech/slsa-builder/internal/handoff"
)

type verifyHandoffCommand struct{}

// NewVerifyHandoffCommand creates the digest-bound one-file handoff subcommand.
func NewVerifyHandoffCommand() Command { return verifyHandoffCommand{} }

func (verifyHandoffCommand) Name() string { return "verify-handoff" }

func (verifyHandoffCommand) Execute(ctx context.Context, args []string, out io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	flags := flag.NewFlagSet("verify-handoff", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	contractPath := flags.String("handoff", "", "typed handoff JSON path")
	artifactDirectory := flags.String("artifact-dir", "", "downloaded one-file artifact directory")
	outputPath := flags.String("output", "", "verified payload output path")
	githubOutput := flags.String("github-output", os.Getenv("GITHUB_OUTPUT"), "GitHub Actions output file")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, writeErr := fmt.Fprintln(out, "Usage: slsa-builder-internal verify-handoff --handoff <json> --artifact-dir <dir> --output <path> [--github-output <path>]")
			return writeErr
		}
		return err
	}
	if flags.NArg() != 0 || *contractPath == "" || *artifactDirectory == "" || *outputPath == "" {
		return errors.New("--handoff, --artifact-dir, and --output are required with no positional arguments")
	}
	contractBytes, err := readRegularFile(*contractPath)
	if err != nil {
		return emitFailure(out, diagnostic.IDInputUnavailable, "handoff.input", "typed handoff input is unavailable", ErrInvocationFailure)
	}
	contract, err := decodeTypedJSON(contractBytes, handoff.Handoff.Validate)
	if err != nil {
		return emitFailure(out, string(handoff.HandoffSchemaMismatchID), "handoff.input", "typed handoff input violates the closed schema", ErrVerificationFailure)
	}
	payload, err := handoff.VerifyOneFile(*artifactDirectory, contract)
	if err != nil {
		id := string(handoff.ErrorIDOf(err))
		if id == "" {
			id = diagnostic.IDInputUnavailable
			return emitFailure(out, id, "handoff.verify", "handoff artifact is unavailable", ErrInvocationFailure)
		}
		return emitFailure(out, id, "handoff.verify", RedactSecrets(err.Error()), ErrVerificationFailure)
	}
	if err := WriteFileAtomic(*outputPath, payload, 0o600); err != nil {
		return emitFailure(out, diagnostic.IDVerifierExecutionFailure, "handoff.output", "verified payload output could not be committed", ErrInvocationFailure)
	}
	if *githubOutput != "" {
		if err := WriteGitHubOutputs(*githubOutput, handoffOutputAllowlist, map[string]string{
			"output-path":    *outputPath,
			"payload-sha256": digest.SumSHA256(payload).String(),
			"result":         "pass",
		}); err != nil {
			return emitFailure(out, diagnostic.IDVerifierExecutionFailure, "github-output", RedactSecrets(err.Error()), ErrInvocationFailure)
		}
	}
	return writeDiagnostics(out, nil, nil)
}

var handoffOutputAllowlist = NewOutputAllowlist("output-path", "payload-sha256", "result")

func emitFailure(out io.Writer, id, check, message string, sentinel error) error {
	if err := writeDiagnosticError(out, id, check, RedactSecrets(message)); err != nil {
		return err
	}
	return sentinel
}
