package command

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/windlasstech/slsa-builder/internal/digest"
	"github.com/windlasstech/slsa-builder/internal/handoff"
	"github.com/windlasstech/slsa-builder/internal/npmprofile"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

var npmProfileBuildOutputs = NewOutputAllowlist(
	"package-name",
	"package-version",
	"package-purl",
	"tarball-name",
	"tarball-path",
	"tarball-artifact-name",
	"tarball-sha256",
	"tarball-sha512",
	"build-metadata-path",
	"build-metadata-artifact-name",
	"build-metadata-sha256",
)

type npmProfileBuildCommand struct{}

// NewNPMProfileBuildCommand creates the npm install, build, and pack subcommand.
func NewNPMProfileBuildCommand() Command { return npmProfileBuildCommand{} }

func (npmProfileBuildCommand) Name() string { return "npm-profile-build" }

func (npmProfileBuildCommand) Execute(ctx context.Context, args []string, out io.Writer) error {
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
	githubOutput := flags.String("github-output", os.Getenv("GITHUB_OUTPUT"), "GitHub Actions output file")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, writeErr := fmt.Fprintln(out, "Usage: slsa-builder-internal npm-profile-build --repository-root <path> --package-directory <path> --observed-repository <owner/repository> --output-directory <path> --artifact-name <name> --metadata-artifact-name <name> [--github-output <path>]")
			return writeErr
		}
		return err
	}
	if *repositoryRoot == "" || *packageDirectory == "" || *observedRepository == "" || *outputDirectory == "" ||
		*artifactName == "" || *metadataArtifactName == "" || *githubOutput == "" || flags.NArg() != 0 {
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
	result, err := npmprofile.BuildPack(ctx, npmprofile.BuildPackConfig{
		Selection:            selection,
		OutputDirectory:      *outputDirectory,
		ArtifactName:         *artifactName,
		ExternalParameters:   json.RawMessage(`{}`),
		ResolvedDependencies: []provenance.ResourceDescriptor{},
	})
	if err != nil {
		return err
	}
	metadataBytes, err := os.ReadFile(result.MetadataPath)
	if err != nil {
		return fmt.Errorf("read build metadata output: %w", err)
	}
	if err := WriteGitHubOutputs(*githubOutput, npmProfileBuildOutputs, map[string]string{
		"package-name":                 result.PackageName,
		"package-version":              result.PackageVersion,
		"package-purl":                 result.PackagePURL,
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
