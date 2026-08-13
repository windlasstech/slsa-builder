package command

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/windlasstech/slsa-builder/internal/diagnostic"
	"github.com/windlasstech/slsa-builder/internal/digest"
	"github.com/windlasstech/slsa-builder/internal/handoff"
	"github.com/windlasstech/slsa-builder/internal/npmprofile"
)

var npmProfileBuildOutputs = NewOutputAllowlist(
	"package-name",
	"package-version",
	"package-purl",
	"package-registry-url",
	"package-url",
	"tarball-name",
	"tarball-path",
	"tarball-artifact-name",
	"tarball-sha256",
	"tarball-sha512",
	"build-metadata-path",
	"build-metadata-artifact-name",
	"build-metadata-sha256",
)

type npmProfileBuildCommand struct {
	httpClient     *http.Client
	runnerOverride *npmprofile.RunnerCapture
}

// NewNPMProfileBuildCommand creates the npm install, build, and pack subcommand.
func NewNPMProfileBuildCommand() Command { return npmProfileBuildCommand{} }

func (npmProfileBuildCommand) Name() string { return "npm-profile-build" }

func (command npmProfileBuildCommand) Execute(ctx context.Context, args []string, out io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	flags := flag.NewFlagSet("npm-profile-build", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	repositoryRoot := flags.String("repository-root", "", "checked-out repository root")
	packageDirectory := flags.String("package-directory", "", "repository-relative package directory")
	observedRepository := flags.String("observed-repository", "", "observed caller repository")
	outputDirectory := flags.String("output-directory", "", "trusted build output directory")
	artifactName := flags.String("artifact-name", "", "tarball handoff artifact name")
	metadataArtifactName := flags.String("metadata-artifact-name", "", "build metadata handoff artifact name")
	registryURL := flags.String("registry-url", "", "caller registry URL intent")
	distTag := flags.String("dist-tag", "", "caller distribution tag intent")
	access := flags.String("access", "", "caller access intent")
	eventName := flags.String("event-name", "", "observed GitHub event name")
	refType := flags.String("ref-type", "", "observed GitHub ref type")
	ref := flags.String("ref", "", "observed GitHub ref")
	revision := flags.String("revision", "", "observed source revision")
	sourceRef := flags.String("source-ref", "", "caller-selected built source tag ref")
	invocationRef := flags.String("invocation-ref", "", "observed caller invocation ref")
	invocationRevision := flags.String("invocation-revision", "", "observed caller invocation revision")
	workflowSHA := flags.String("workflow-sha", "", "immutable reusable workflow revision")
	callerWorkflow := flags.String("caller-workflow-filename", "", "observed caller workflow filename")
	githubOutput := flags.String("github-output", os.Getenv("GITHUB_OUTPUT"), "GitHub Actions output file")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, writeErr := fmt.Fprintln(out, "Usage: slsa-builder-internal npm-profile-build --repository-root <path> --package-directory <path> --observed-repository <owner/repository> --output-directory <path> --artifact-name <name> --metadata-artifact-name <name> [--github-output <path>]")
			return writeErr
		}
		return err
	}
	normalizedSourceRef := npmprofile.NormalizeSourceRefInput(*sourceRef)
	if *repositoryRoot == "" || *packageDirectory == "" || *observedRepository == "" || *outputDirectory == "" ||
		*artifactName == "" || *metadataArtifactName == "" || *eventName == "" || *refType == "" || *ref == "" ||
		*revision == "" || *workflowSHA == "" || *callerWorkflow == "" || *githubOutput == "" || flags.NArg() != 0 {
		return errors.New("all npm-profile-build flags are required with no positional arguments")
	}
	if err := handoff.ValidateSafeBasename(*artifactName); err != nil {
		return fmt.Errorf("invalid tarball artifact name: %w", err)
	}
	if err := handoff.ValidateSafeBasename(*metadataArtifactName); err != nil {
		return fmt.Errorf("invalid build metadata artifact name: %w", err)
	}

	selection, err := npmprofile.Analyze(npmprofile.Config{
		RepositoryRoot:     *repositoryRoot,
		PackageDirectory:   *packageDirectory,
		ObservedRepository: *observedRepository,
	})
	if err != nil {
		return err
	}
	if selection.Report.ExitCode != 0 {
		if err := WriteReport(out, selection.Report); err != nil {
			return err
		}
		return ErrVerificationFailure
	}
	if err := npmprofile.ValidateSourceRefInput(normalizedSourceRef, *invocationRef, selection.Package.Version); err != nil {
		if reportErr := writeNPMBuildPolicyError(out, err); reportErr != nil {
			return reportErr
		}
		return ErrVerificationFailure
	}
	publishIntent, err := npmprofile.ResolveWorkflowPublishIntent(selection, *registryURL, *distTag, *access)
	if err != nil {
		return err
	}
	registry, err := npmprofile.NewRegistryClient(npmprofile.RegistryClientConfig{HTTPClient: command.httpClient, RegistryURL: publishIntent.ResolvedRegistryURL})
	if err != nil {
		return err
	}
	registryState, err := registry.Preflight(ctx, selection.Package.Name, selection.Package.Version)
	if err != nil {
		return err
	}
	result, err := npmprofile.BuildPack(ctx, npmprofile.BuildPackConfig{
		Selection:          selection,
		OutputDirectory:    *outputDirectory,
		ArtifactName:       *artifactName,
		ExternalParameters: json.RawMessage(`{}`),
	})
	if err != nil {
		return err
	}
	if command.runnerOverride != nil {
		result.Toolchain.Runner = *command.runnerOverride
	}
	metadata, err := npmprofile.FinalizeWorkflowBuildMetadata(selection, result, npmprofile.WorkflowBuildMetadataConfig{
		ArtifactName: *artifactName, RegistryURLInput: *registryURL, DistTagInput: *distTag, AccessInput: *access,
		EventName: *eventName, RefType: *refType, Ref: *ref, Revision: *revision, SourceRefInput: normalizedSourceRef,
		InvocationRef: conditionalSourceIdentity(normalizedSourceRef, *invocationRef), InvocationRevision: conditionalSourceIdentity(normalizedSourceRef, *invocationRevision), WorkflowSHA: *workflowSHA,
		CallerWorkflowFilename: *callerWorkflow, RegistryState: registryState,
	})
	if err != nil {
		return err
	}
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	if err := WriteFileAtomic(result.MetadataPath, metadataBytes, 0o600); err != nil {
		return err
	}
	metadataBytes, err = os.ReadFile(result.MetadataPath)
	if err != nil {
		return fmt.Errorf("read build metadata output: %w", err)
	}
	if err := WriteGitHubOutputs(*githubOutput, npmProfileBuildOutputs, map[string]string{
		"package-name":                 result.PackageName,
		"package-version":              result.PackageVersion,
		"package-purl":                 result.PackagePURL,
		"package-registry-url":         publishIntent.ResolvedRegistryURL,
		"package-url":                  metadataPackageURL(metadata),
		"tarball-name":                 filepath.Base(result.TarballPath),
		"tarball-path":                 result.TarballPath,
		"tarball-artifact-name":        *artifactName,
		"tarball-sha256":               result.SHA256.String(),
		"tarball-sha512":               result.SHA512.String(),
		"build-metadata-path":          result.MetadataPath,
		"build-metadata-artifact-name": *metadataArtifactName,
		"build-metadata-sha256":        digest.SumSHA256(metadataBytes).String(),
	}); err != nil {
		return err
	}
	return writeDiagnostics(out, nil, nil)
}

func writeNPMBuildPolicyError(out io.Writer, err error) error {
	var identified interface{ DiagnosticID() string }
	if !errors.As(err, &identified) || identified.DiagnosticID() != npmprofile.IDSourceRefInvalid {
		return err
	}
	entry, buildErr := diagnostic.New(identified.DiagnosticID(), "source-ref", err.Error())
	if buildErr != nil {
		return buildErr
	}
	report, buildErr := diagnostic.Build(nil, []diagnostic.Diagnostic{entry}, nil)
	if buildErr != nil {
		return buildErr
	}
	return WriteReport(out, report)
}

func conditionalSourceIdentity(sourceRef, value string) string {
	if sourceRef == "" {
		return ""
	}
	return value
}

func metadataPackageURL(metadata npmprofile.BuildMetadata) string {
	parameters, err := npmprofile.DecodeExternalParameters(metadata.ExternalParameters)
	if err != nil {
		return ""
	}
	return parameters.Package.PackageURL
}
