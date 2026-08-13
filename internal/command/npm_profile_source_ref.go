package command

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/windlasstech/slsa-builder/internal/diagnostic"
	"github.com/windlasstech/slsa-builder/internal/identity"
	"github.com/windlasstech/slsa-builder/internal/npmprofile"
)

var npmProfileSourceRefOutputs = NewOutputAllowlist("built-ref", "built-revision")

type npmProfileSourceRefCommand struct{}

// NewNPMProfileSourceRefCommand creates the pre-checkout source-ref validation and resolution subcommand.
func NewNPMProfileSourceRefCommand() Command { return npmProfileSourceRefCommand{} }

func (npmProfileSourceRefCommand) Name() string { return "npm-profile-source-ref" }

func (npmProfileSourceRefCommand) Execute(ctx context.Context, args []string, out io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	flags := flag.NewFlagSet("npm-profile-source-ref", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	sourceRef := flags.String("source-ref", "", "caller built source ref intent")
	invocationRef := flags.String("ref", "", "observed invocation ref")
	refType := flags.String("ref-type", "", "observed GitHub ref type")
	invocationRevision := flags.String("revision", "", "observed invocation revision")
	observedRepository := flags.String("observed-repository", "", "observed caller repository")
	githubOutput := flags.String("github-output", os.Getenv("GITHUB_OUTPUT"), "GitHub Actions output file")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, writeErr := fmt.Fprintln(out, "Usage: slsa-builder-internal npm-profile-source-ref --ref <ref> --ref-type <type> --revision <sha> --observed-repository <owner/repository> [--source-ref <ref>] [--github-output <path>]")
			return writeErr
		}
		return err
	}
	if *invocationRef == "" || *refType == "" || *invocationRevision == "" || *observedRepository == "" || flags.NArg() != 0 {
		return errors.New("--ref, --ref-type, --revision, and --observed-repository are required with no positional arguments")
	}

	builtRef, builtRevision := *invocationRef, *invocationRevision
	if trimmed := strings.Trim(*sourceRef, " \t\r\n\f\v"); trimmed != "" {
		if err := npmprofile.ValidateSourceRefInput(trimmed, *invocationRef); err != nil {
			return emitSourceRefFailure(out, err)
		}
		remote, err := identity.CanonicalRepository(*observedRepository)
		if err != nil {
			return emitSourceRefFailure(out, err)
		}
		revision, err := npmprofile.ResolveSourceRefTag(ctx, remote, trimmed)
		if err != nil {
			return emitSourceRefFailure(out, err)
		}
		builtRef, builtRevision = trimmed, revision
	}
	if err := WriteGitHubOutputs(*githubOutput, npmProfileSourceRefOutputs, map[string]string{
		"built-ref":      builtRef,
		"built-revision": builtRevision,
	}); err != nil {
		return err
	}
	return writeDiagnostics(out, nil, nil)
}

func emitSourceRefFailure(out io.Writer, err error) error {
	entry, ok := npmprofile.DiagnosticOf(err)
	if !ok {
		return err
	}
	if writeErr := writeDiagnostics(out, nil, []diagnostic.Diagnostic{entry}); writeErr != nil {
		return writeErr
	}
	return ErrVerificationFailure
}
