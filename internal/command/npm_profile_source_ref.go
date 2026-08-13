package command

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/windlasstech/slsa-builder/internal/diagnostic"
	"github.com/windlasstech/slsa-builder/internal/npmprofile"
)

var (
	npmSourceRefOutputs = NewOutputAllowlist("built-ref", "built-revision")
	githubRepository    = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
)

type sourceRefResolver func(context.Context, string, string, string) (string, error)

type npmProfileSourceRefCommand struct {
	resolve sourceRefResolver
}

// NewNPMProfileSourceRefCommand creates the pre-checkout npm source-ref resolver subcommand.
func NewNPMProfileSourceRefCommand() Command {
	return newNPMProfileSourceRefCommand(npmprofile.ResolveSourceRefTag)
}

func newNPMProfileSourceRefCommand(resolve sourceRefResolver) Command {
	return npmProfileSourceRefCommand{resolve: resolve}
}

func (npmProfileSourceRefCommand) Name() string {
	return "npm-profile-source-ref"
}

func (command npmProfileSourceRefCommand) Execute(ctx context.Context, args []string, out io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	flags := flag.NewFlagSet(command.Name(), flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	sourceRef := flags.String("source-ref", "", "full caller-selected tag ref")
	invocationRef := flags.String("ref", "", "invocation ref observation")
	refType := flags.String("ref-type", "", "invocation ref type observation")
	revision := flags.String("revision", "", "invocation revision observation")
	observedRepository := flags.String("observed-repository", "", "canonical caller repository observation")
	githubOutput := flags.String("github-output", os.Getenv("GITHUB_OUTPUT"), "GitHub output file")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, writeErr := fmt.Fprintln(out, "Usage: slsa-builder-internal npm-profile-source-ref --source-ref <ref> --ref <ref> --ref-type <type> --revision <sha> --observed-repository <owner/repository> --github-output <path>")
			return writeErr
		}
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", flags.Args())
	}
	if *invocationRef == "" || *refType == "" || *revision == "" || *observedRepository == "" || *githubOutput == "" {
		return errors.New("--ref, --ref-type, --revision, --observed-repository, and --github-output are required")
	}
	if !validSourceObservation(*invocationRef, *refType, *revision, *observedRepository) {
		return errors.New("source observations must be canonical GitHub ref, type, revision, and repository values")
	}

	builtRef := *invocationRef
	builtRevision := *revision
	if strings.Trim(*sourceRef, " \t\n\r\v\f") != "" {
		if err := npmprofile.ValidateSourceRefInput(*sourceRef, *invocationRef, ""); err != nil {
			return writeSourceRefFailure(out, err)
		}
		remote := "https://github.com/" + *observedRepository + ".git"
		resolved, err := command.resolve(ctx, *sourceRef, *invocationRef, remote)
		if err != nil {
			return writeSourceRefFailure(out, err)
		}
		builtRef = *sourceRef
		builtRevision = resolved
	}
	if err := WriteGitHubOutputs(*githubOutput, npmSourceRefOutputs, map[string]string{
		"built-ref":      builtRef,
		"built-revision": builtRevision,
	}); err != nil {
		return err
	}
	return nil
}

func validSourceObservation(ref, refType, revision, repository string) bool {
	if !strings.HasPrefix(ref, "refs/") || strings.ContainsAny(ref, "\r\n\x00") {
		return false
	}
	if refType != "branch" && refType != "tag" {
		return false
	}
	if len(revision) != 40 || strings.Trim(revision, "0123456789abcdef") != "" {
		return false
	}
	return githubRepository.MatchString(repository) && !strings.Contains(repository, "..")
}

func writeSourceRefFailure(out io.Writer, cause error) error {
	typed, ok := cause.(interface{ DiagnosticID() string })
	if !ok || typed.DiagnosticID() != npmprofile.IDSourceRefInvalid {
		return cause
	}
	entry, err := diagnostic.New(typed.DiagnosticID(), "source-ref.resolve", cause.Error())
	if err != nil {
		return err
	}
	entry.Field = "source-ref"
	report, err := diagnostic.Build(nil, []diagnostic.Diagnostic{entry}, nil)
	if err != nil {
		return err
	}
	if err := WriteReport(out, report); err != nil {
		return err
	}
	return ErrVerificationFailure
}
