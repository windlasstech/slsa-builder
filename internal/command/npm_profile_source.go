package command

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"

	"github.com/windlasstech/slsa-builder/internal/npmprofile"
)

var npmProfileSourceOutputs = NewOutputAllowlist("ref", "ref-type", "revision")

type npmProfileSourceCommand struct{}

// NewNPMProfileSourceCommand creates the immutable source selection command.
func NewNPMProfileSourceCommand() Command { return npmProfileSourceCommand{} }

func (npmProfileSourceCommand) Name() string { return "npm-profile-source" }

func (npmProfileSourceCommand) Execute(ctx context.Context, args []string, out io.Writer) error {
	flags := flag.NewFlagSet("npm-profile-source", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	sourceRef := flags.String("source-ref", "", "optional full release tag ref")
	eventRef := flags.String("event-ref", "", "trusted event ref")
	eventRevision := flags.String("event-revision", "", "trusted event revision")
	eventRefType := flags.String("event-ref-type", "", "trusted event ref type")
	repository := flags.String("repository", "", "caller repository")
	apiURL := flags.String("api-url", "", "GitHub API root")
	githubOutput := flags.String("github-output", os.Getenv("GITHUB_OUTPUT"), "GitHub Actions output file")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *eventRef == "" || *eventRevision == "" || *eventRefType == "" || *repository == "" || *apiURL == "" || *githubOutput == "" {
		return errors.New("all npm-profile-source event flags are required with no positional arguments")
	}
	resolved, err := npmprofile.ResolveSourceRef(ctx, npmprofile.SourceRefResolutionConfig{
		APIURL: *apiURL, Token: os.Getenv("GITHUB_TOKEN"), Repository: *repository,
		SourceRef: *sourceRef, EventRef: *eventRef, EventRevision: *eventRevision, EventRefType: *eventRefType,
	})
	if err != nil {
		return err
	}
	if err := WriteGitHubOutputs(*githubOutput, npmProfileSourceOutputs, map[string]string{
		"ref": resolved.Ref, "ref-type": resolved.RefType, "revision": resolved.Revision,
	}); err != nil {
		return err
	}
	return writeDiagnostics(out, nil, nil)
}
