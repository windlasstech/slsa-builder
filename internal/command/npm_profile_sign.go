package command

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/windlasstech/slsa-builder/internal/attestation"
	"github.com/windlasstech/slsa-builder/internal/digest"
	"github.com/windlasstech/slsa-builder/internal/handoff"
	"github.com/windlasstech/slsa-builder/internal/npmprofile"
	"github.com/windlasstech/slsa-builder/internal/signing"
)

var npmProfileSignOutputs = NewOutputAllowlist("bundle-path", "bundle-sha256", "result", "statement-sha256")

type npmProfileSignCommand struct{}

// NewNPMProfileSignCommand creates the digest-bound Go-native npm provenance signer.
func NewNPMProfileSignCommand() Command { return npmProfileSignCommand{} }

func (npmProfileSignCommand) Name() string { return "npm-profile-sign" }

func (npmProfileSignCommand) Execute(ctx context.Context, args []string, out io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	flags := flag.NewFlagSet("npm-profile-sign", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	metadataDirectory := flags.String("metadata-artifact-dir", "", "downloaded build metadata artifact directory")
	metadataDigest := flags.String("metadata-sha256", "", "expected build metadata SHA-256")
	metadataArtifactName := flags.String("metadata-artifact-name", "", "expected build metadata artifact name")
	tarballDirectory := flags.String("tarball-artifact-dir", "", "downloaded tarball artifact directory")
	tarballDigest := flags.String("tarball-sha256", "", "expected tarball SHA-256")
	tarballArtifactName := flags.String("tarball-artifact-name", "", "expected tarball artifact name")
	nodeVersion := flags.String("node-version", "", "observed Node.js version")
	corepackVersion := flags.String("corepack-version", "", "observed Corepack version when used")
	registryURL := flags.String("registry-url", "", "resolved npm registry root")
	packageName := flags.String("package-name", "", "selected package name")
	outputDirectory := flags.String("output-directory", "", "trusted provenance output directory")
	githubOutput := flags.String("github-output", os.Getenv("GITHUB_OUTPUT"), "GitHub Actions output file")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, writeErr := fmt.Fprintln(out, "Usage: slsa-builder-internal npm-profile-sign --metadata-artifact-dir <dir> --metadata-sha256 <hex> --metadata-artifact-name <name> --tarball-artifact-dir <dir> --tarball-sha256 <hex> --tarball-artifact-name <name> --node-version <version> [--corepack-version <version>] --output-directory <dir>")
			return writeErr
		}
		return err
	}
	if flags.NArg() != 0 || *metadataDirectory == "" || *metadataDigest == "" || *metadataArtifactName == "" ||
		*tarballDirectory == "" || *tarballDigest == "" || *tarballArtifactName == "" || *nodeVersion == "" ||
		*registryURL == "" || *packageName == "" || *outputDirectory == "" || *githubOutput == "" {
		return errors.New("all required npm-profile-sign flags must be supplied with no positional arguments")
	}
	metadataSHA256, err := digest.ParseSHA256(*metadataDigest)
	if err != nil {
		return errors.New("metadata SHA-256 is invalid")
	}
	tarballSHA256, err := digest.ParseSHA256(*tarballDigest)
	if err != nil {
		return errors.New("tarball SHA-256 is invalid")
	}
	metadataBytes, err := handoff.VerifyOneFile(*metadataDirectory, handoff.Handoff{
		Transport:       handoff.TransportGitHubActionsArtifact,
		ArtifactName:    *metadataArtifactName,
		PayloadFileName: "build-metadata.json",
		PayloadKind:     "application/vnd.windlass.npm-build-metadata.v1+json",
		Digest:          handoff.Digest{Algorithm: handoff.AlgorithmSHA256, Value: metadataSHA256},
	})
	if err != nil {
		return fmt.Errorf("verify build metadata handoff: %w", err)
	}
	metadata, err := decodeTypedJSON[npmprofile.BuildMetadata](metadataBytes, nil)
	if err != nil {
		return fmt.Errorf("decode build metadata: %w", err)
	}
	tarballBytes, err := handoff.VerifyOneFile(*tarballDirectory, handoff.Handoff{
		Transport:       handoff.TransportGitHubActionsArtifact,
		ArtifactName:    *tarballArtifactName,
		PayloadFileName: metadata.PrimaryArtifact.PayloadFileName,
		PayloadKind:     "application/vnd.npm.package-tar+gzip",
		Digest:          handoff.Digest{Algorithm: handoff.AlgorithmSHA256, Value: tarballSHA256},
	})
	if err != nil {
		return fmt.Errorf("verify tarball handoff: %w", err)
	}
	if metadata.PrimaryArtifact.ArtifactName != *tarballArtifactName || metadata.PrimaryArtifact.SHA256 != tarballSHA256.String() ||
		metadata.PrimaryArtifact.SHA512 != digest.SumSHA512(tarballBytes).String() {
		return errors.New("verified tarball differs from build metadata")
	}
	parameters, err := npmprofile.DecodeExternalParameters(metadata.ExternalParameters)
	if err != nil {
		return err
	}
	if parameters.Package.Name != *packageName || parameters.Publish.ResolvedRegistryURL != *registryURL {
		return errors.New("signing preflight inputs differ from signed build metadata")
	}
	oidc, err := npmprofile.NewOIDCClient(npmprofile.OIDCClientConfig{
		RegistryURL:         *registryURL,
		IDTokenRequestURL:   os.Getenv("ACTIONS_ID_TOKEN_REQUEST_URL"),
		IDTokenRequestToken: os.Getenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN"),
		GitHubWorkflowRef:   os.Getenv("GITHUB_WORKFLOW_REF"),
	})
	if err != nil {
		return err
	}
	preflight := oidc.Preflight(ctx, *packageName)
	if preflight.Report.PrimaryID != nil {
		if err := WriteReport(out, preflight.Report); err != nil {
			return err
		}
		return ErrVerificationFailure
	}
	if preflight.WorkflowFilename != parameters.Caller.WorkflowFilename {
		return errors.New("OIDC caller workflow differs from signed build metadata")
	}
	observedCorepack := (*string)(nil)
	if parameters.PackageManager.Name != npmprofile.ManagerNPM {
		if *corepackVersion == "" {
			return errors.New("corepack version is required for pnpm and Yarn")
		}
		observedCorepack = corepackVersion
	}
	now := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	runInvocation := githubRunInvocationURI()
	signingInput, err := npmprofile.NewProvenanceSigningInput(npmprofile.NPMProvenanceInput{
		BuildMetadata:         metadata,
		BuilderID:             parameters.Workflow.BuilderID,
		NodeJSVersion:         *nodeVersion,
		CorepackVersion:       observedCorepack,
		InvocationID:          runInvocation,
		StartedOn:             now,
		FinishedOn:            now,
		RuntimeReleaseRef:     os.Getenv("GITHUB_REF"),
		PeeledReleaseRevision: os.Getenv("GITHUB_SHA"),
	})
	if err != nil {
		return err
	}
	identity, err := githubSigningIdentity(runInvocation)
	if err != nil {
		return err
	}
	result, err := signing.SignGitHubActions(ctx, signing.Request{Statement: signingInput.StatementJSON, Identity: identity})
	if err != nil {
		return err
	}
	if err := ensureEmptyDirectory(*outputDirectory); err != nil {
		return err
	}
	bundlePath := filepath.Join(*outputDirectory, metadata.PrimaryArtifact.PayloadFileName+".intoto.jsonl")
	if err := WriteFileAtomic(bundlePath, result.Bundle, 0o600); err != nil {
		return fmt.Errorf("write verified provenance bundle: %w", err)
	}
	if err := WriteGitHubOutputs(*githubOutput, npmProfileSignOutputs, map[string]string{
		"bundle-path":      bundlePath,
		"bundle-sha256":    digest.SumSHA256(result.Bundle).String(),
		"result":           "pass",
		"statement-sha256": digest.SumSHA256(result.Statement).String(),
	}); err != nil {
		return err
	}
	return writeDiagnostics(out, &runInvocation, nil)
}

func githubRunInvocationURI() string {
	return os.Getenv("GITHUB_SERVER_URL") + "/" + os.Getenv("GITHUB_REPOSITORY") + "/actions/runs/" + os.Getenv("GITHUB_RUN_ID") + "/attempts/" + os.Getenv("GITHUB_RUN_ATTEMPT")
}

func githubSigningIdentity(runInvocation string) (attestation.IdentityExpectation, error) {
	required := []string{"GITHUB_SERVER_URL", "GITHUB_REPOSITORY", "GITHUB_REPOSITORY_ID", "GITHUB_REPOSITORY_OWNER_ID", "GITHUB_SHA", "GITHUB_REF", "WINDLASS_WORKFLOW_SHA"}
	for _, name := range required {
		if os.Getenv(name) == "" {
			return attestation.IdentityExpectation{}, fmt.Errorf("required trusted runtime value %s is unavailable", name)
		}
	}
	return attestation.IdentityExpectation{
		Issuer:                  "https://token.actions.githubusercontent.com",
		WorkflowSHA:             os.Getenv("WINDLASS_WORKFLOW_SHA"),
		SourceRepositoryURI:     os.Getenv("GITHUB_SERVER_URL") + "/" + os.Getenv("GITHUB_REPOSITORY"),
		SourceRepositoryID:      os.Getenv("GITHUB_REPOSITORY_ID"),
		SourceRepositoryOwnerID: os.Getenv("GITHUB_REPOSITORY_OWNER_ID"),
		SourceDigest:            os.Getenv("GITHUB_SHA"),
		SourceRef:               os.Getenv("GITHUB_REF"),
		RunnerEnvironment:       "github-hosted",
		RunInvocationURI:        runInvocation,
	}, nil
}

func ensureEmptyDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("provenance output directory must be an existing non-symlink directory")
	}
	entries, err := os.ReadDir(path)
	if err != nil || len(entries) != 0 {
		return errors.New("provenance output directory must be readable and empty")
	}
	return nil
}
