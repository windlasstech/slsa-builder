package command

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/windlasstech/slsa-builder/internal/attestation"
	"github.com/windlasstech/slsa-builder/internal/diagnostic"
	"github.com/windlasstech/slsa-builder/internal/digest"
	"github.com/windlasstech/slsa-builder/internal/handoff"
	"github.com/windlasstech/slsa-builder/internal/npmprofile"
	"github.com/windlasstech/slsa-builder/internal/policy"
)

var npmProfilePublishOutputs = NewOutputAllowlist("package-name", "package-version", "package-registry-url", "package-url", "package-tarball-name", "package-tarball-sha256", "package-tarball-sha512", "report-path", "result")

type npmProfilePublishInput struct {
	TarballPath, BundlePath, NPMExecutable, RegistryURL, PackageName string
	TarballSHA256                                                    digest.SHA256
	TarballSHA512                                                    digest.SHA512
	BundleSHA256                                                     digest.SHA256
}

type npmProfilePublishCommand struct {
	publish func(context.Context, npmProfilePublishInput) (npmprofile.PublishResult, error)
}

// NewNPMProfilePublishCommand creates the serialized npm publish convergence command.
func NewNPMProfilePublishCommand() Command { return npmProfilePublishCommand{} }

func (npmProfilePublishCommand) Name() string { return "npm-profile-publish" }

func (command npmProfilePublishCommand) Execute(ctx context.Context, args []string, out io.Writer) error {
	flags := flag.NewFlagSet("npm-profile-publish", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	tarballDir := flags.String("tarball-artifact-dir", "", "downloaded tarball artifact directory")
	tarballArtifact := flags.String("tarball-artifact-name", "", "tarball artifact name")
	tarballName := flags.String("tarball-name", "", "tarball file name")
	tarball256Text := flags.String("tarball-sha256", "", "tarball SHA-256")
	tarball512Text := flags.String("tarball-sha512", "", "tarball SHA-512")
	bundleDir := flags.String("bundle-artifact-dir", "", "downloaded provenance artifact directory")
	bundleArtifact := flags.String("bundle-artifact-name", "", "provenance artifact name")
	bundleName := flags.String("bundle-name", "", "provenance bundle file name")
	bundle256Text := flags.String("bundle-sha256", "", "bundle SHA-256")
	packageName := flags.String("package-name", "", "signed package name")
	packageVersion := flags.String("package-version", "", "signed package version")
	packageURL := flags.String("package-url", "", "signed package registry URL")
	registryURL := flags.String("registry-url", "", "resolved registry root")
	npmExecutable := flags.String("npm-executable", "", "absolute npm executable path")
	reportPath := flags.String("report-path", "", "persistent outcome report path")
	githubOutput := flags.String("github-output", os.Getenv("GITHUB_OUTPUT"), "GitHub Actions output file")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *tarballDir == "" || *tarballArtifact == "" || *tarballName == "" || *tarball256Text == "" || *tarball512Text == "" || *bundleDir == "" || *bundleArtifact == "" || *bundleName == "" || *bundle256Text == "" || *packageName == "" || *packageVersion == "" || *packageURL == "" || *registryURL == "" || *npmExecutable == "" || *reportPath == "" || *githubOutput == "" {
		return errors.New("all npm-profile-publish flags are required with no positional arguments")
	}
	tarball256, err := digest.ParseSHA256(*tarball256Text)
	if err != nil {
		return err
	}
	tarball512, err := digest.ParseSHA512(*tarball512Text)
	if err != nil {
		return err
	}
	bundle256, err := digest.ParseSHA256(*bundle256Text)
	if err != nil {
		return err
	}
	if _, err := handoff.VerifyOneFile(*tarballDir, handoff.Handoff{Transport: handoff.TransportGitHubActionsArtifact, ArtifactName: *tarballArtifact, PayloadFileName: *tarballName, PayloadKind: "application/vnd.npm.package-tar+gzip", Digest: handoff.Digest{Algorithm: handoff.AlgorithmSHA256, Value: tarball256}}); err != nil {
		return fmt.Errorf("verify tarball handoff: %w", err)
	}
	if _, err := handoff.VerifyOneFile(*bundleDir, handoff.Handoff{Transport: handoff.TransportGitHubActionsArtifact, ArtifactName: *bundleArtifact, PayloadFileName: *bundleName, PayloadKind: "application/vnd.dev.sigstore.bundle+json", Digest: handoff.Digest{Algorithm: handoff.AlgorithmSHA256, Value: bundle256}}); err != nil {
		return fmt.Errorf("verify provenance handoff: %w", err)
	}
	input := npmProfilePublishInput{TarballPath: filepath.Join(*tarballDir, *tarballName), BundlePath: filepath.Join(*bundleDir, *bundleName), NPMExecutable: *npmExecutable, RegistryURL: *registryURL, PackageName: *packageName, TarballSHA256: tarball256, TarballSHA512: tarball512, BundleSHA256: bundle256}
	publish := command.publish
	if publish == nil {
		publish = runNPMProfilePublish
	}
	result, publishErr := publish(ctx, input)
	encoded, reportErr := result.Report.CanonicalJSON()
	if reportErr == nil {
		reportErr = WriteFileAtomic(*reportPath, encoded, 0o600)
	}
	if reportErr != nil {
		return reportErr
	}
	status := "pass"
	if publishErr != nil {
		status = "fail"
	}
	if err := WriteGitHubOutputs(*githubOutput, npmProfilePublishOutputs, map[string]string{"package-name": *packageName, "package-version": *packageVersion, "package-registry-url": *registryURL, "package-url": *packageURL, "package-tarball-name": *tarballName, "package-tarball-sha256": tarball256.String(), "package-tarball-sha512": tarball512.String(), "report-path": *reportPath, "result": status}); err != nil {
		return err
	}
	if err := WriteReport(out, result.Report); err != nil {
		return err
	}
	if publishErr != nil {
		return ErrVerificationFailure
	}
	return nil
}

func runNPMProfilePublish(ctx context.Context, input npmProfilePublishInput) (npmprofile.PublishResult, error) {
	oidc, err := npmprofile.NewOIDCClient(npmprofile.OIDCClientConfig{RegistryURL: input.RegistryURL, IDTokenRequestURL: os.Getenv("ACTIONS_ID_TOKEN_REQUEST_URL"), IDTokenRequestToken: os.Getenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN"), GitHubWorkflowRef: os.Getenv("GITHUB_WORKFLOW_REF")})
	if err != nil {
		return publishInvocationFailure(err)
	}
	exchange := oidc.Preflight(ctx, input.PackageName)
	if exchange.Report.PrimaryID != nil {
		return npmprofile.PublishResult{State: npmprofile.PublishIndeterminate, Report: exchange.Report}, errors.New("trusted-publisher preflight failed")
	}
	identity, err := githubSigningIdentity(githubRunInvocationURI())
	if err != nil {
		return publishInvocationFailure(err)
	}
	identity.SignerURI = "https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@" + os.Getenv("WINDLASS_WORKFLOW_SHA")
	registry, err := npmprofile.NewRegistryClient(npmprofile.RegistryClientConfig{RegistryURL: input.RegistryURL})
	if err != nil {
		return publishInvocationFailure(err)
	}
	verifier := npmprofile.NewSigstorePublishBundleVerifier(npmprofile.SigstorePublishVerifierConfig{Mode: attestation.ModeOnline, TrustRoot: policy.TrustRoot{Mode: "tuf", Instance: "sigstore-public-good"}, Identity: identity})
	return npmprofile.Publish(ctx, npmprofile.PublishRequest{NPMExecutable: input.NPMExecutable, TarballPath: input.TarballPath, BundlePath: input.BundlePath, TarballSHA256: input.TarballSHA256, TarballSHA512: input.TarballSHA512, BundleSHA256: input.BundleSHA256, Registry: registry, RunID: os.Getenv("GITHUB_RUN_ID"), RunAttempt: os.Getenv("GITHUB_RUN_ATTEMPT"), SourceRepositoryURI: identity.SourceRepositoryURI, OIDCExchange: exchange, Verifier: verifier})
}

func publishInvocationFailure(cause error) (npmprofile.PublishResult, error) {
	entry, err := diagnostic.New(diagnostic.IDVerifierExecutionFailure, "npm.publish", cause.Error())
	if err != nil {
		panic(err)
	}
	report, err := diagnostic.Build(nil, []diagnostic.Diagnostic{entry}, nil)
	if err != nil {
		panic(err)
	}
	return npmprofile.PublishResult{State: npmprofile.PublishIndeterminate, Report: report}, cause
}
