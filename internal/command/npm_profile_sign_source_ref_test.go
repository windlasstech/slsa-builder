package command

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/windlasstech/slsa-builder/internal/digest"
	"github.com/windlasstech/slsa-builder/internal/identity"
	"github.com/windlasstech/slsa-builder/internal/npmprofile"
	"github.com/windlasstech/slsa-builder/internal/provenance"
	"github.com/windlasstech/slsa-builder/internal/signing"
)

const (
	signBuiltRef             = "refs/tags/v1.2.3"
	signBuiltRevision        = "0123456789abcdef0123456789abcdef01234567"
	signInvocationRef        = "refs/heads/main"
	signInvocationRevision   = "89abcdef0123456789abcdef0123456789abcdef"
	signOtherBuiltRevision   = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	signMetadataArtifactName = "js-ts-npm-build-metadata-123456789-1"
	signTarballArtifactName  = "js-ts-npm-package-tarball-123456789-1"
	signTarballName          = "windlass-slsa-builder-1.2.3.tgz"
	signBuilderWorkflowSHA   = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	signRunInvocation        = "https://github.com/example/project/actions/runs/123456789/attempts/1"
	signRegistryURL          = "https://registry.npmjs.org/"
	signPackageName          = "@windlass/slsa-builder"
)

func TestNPMProfileSignCommandSourceRefDispatchRetry(t *testing.T) {
	metadata, tarball := signSourceRefMetadata(t)
	metadataDirectory := writeSignHandoff(t, "build-metadata.json", mustMarshalSignJSON(t, metadata))
	tarballDirectory := writeSignHandoff(t, signTarballName, tarball)
	outputDirectory := t.TempDir()
	githubOutput := filepath.Join(t.TempDir(), "github-output")
	setSignGitHubEnvironment(t, signInvocationRef, signInvocationRevision, githubOutput)

	signCalled := false
	command := npmProfileSignCommand{
		preflight: func(ctx context.Context, registryURL, packageName string) npmprofile.OIDCExchangeResult {
			if err := ctx.Err(); err != nil {
				t.Fatal(err)
			}
			if registryURL != signRegistryURL || packageName != signPackageName {
				t.Fatalf("preflight inputs = %q, %q", registryURL, packageName)
			}
			return npmprofile.OIDCExchangeResult{WorkflowFilename: "release.yml"}
		},
		sign: func(ctx context.Context, request signing.Request) (signing.Result, error) {
			if err := ctx.Err(); err != nil {
				t.Fatal(err)
			}
			signCalled = true
			statement, err := provenance.DecodeStatement(request.Statement)
			if err != nil {
				t.Fatalf("decode signing request: %v", err)
			}
			parameters, err := npmprofile.DecodeExternalParameters(statement.Predicate.BuildDefinition.ExternalParameters)
			if err != nil {
				t.Fatalf("decode signed external parameters: %v", err)
			}
			if parameters.Source.Ref != signBuiltRef || parameters.Source.Revision != signBuiltRevision ||
				parameters.Release.Ref != signBuiltRef {
				t.Fatalf("signed built release identity = source %#v, release %#v", parameters.Source, parameters.Release)
			}
			if parameters.Source.InvocationRef == nil || *parameters.Source.InvocationRef != signInvocationRef ||
				parameters.Source.InvocationRevision == nil || *parameters.Source.InvocationRevision != signInvocationRevision {
				t.Fatalf("signed invocation record = %#v", parameters.Source)
			}
			if err := npmprofile.ValidateReleaseRefEquality(
				parameters.Source.Ref,
				parameters.Release.Ref,
				signBuiltRef,
				parameters.Release.VersionTag,
				parameters.Source.Revision,
				signBuiltRevision,
			); err != nil {
				t.Fatalf("built release equality: %v", err)
			}
			if request.Identity.SourceRef != signInvocationRef || request.Identity.SourceDigest != signInvocationRevision {
				t.Fatalf("certificate context = %#v", request.Identity)
			}
			return signing.Result{Statement: append([]byte(nil), request.Statement...), Bundle: []byte(`{"bundle":"fixture"}`)}, nil
		},
		now: func() time.Time { return time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC) },
	}

	var output bytes.Buffer
	err := command.Execute(context.Background(), signSourceRefArguments(
		metadataDirectory,
		tarballDirectory,
		outputDirectory,
		githubOutput,
		digest.SumSHA256(mustMarshalSignJSON(t, metadata)).String(),
		digest.SumSHA256(tarball).String(),
		signBuiltRef,
		signBuiltRevision,
	), &output)
	if err != nil {
		t.Fatalf("Execute() error = %v, output = %s", err, output.String())
	}
	if !signCalled {
		t.Fatal("signer was not called")
	}
}

func TestNPMProfileSignCommandRejectsMismatchedBuiltIdentity(t *testing.T) {
	metadata, tarball := signSourceRefMetadata(t)
	metadataBytes := mustMarshalSignJSON(t, metadata)
	metadataDirectory := writeSignHandoff(t, "build-metadata.json", metadataBytes)
	tarballDirectory := writeSignHandoff(t, signTarballName, tarball)
	setSignGitHubEnvironment(t, signInvocationRef, signInvocationRevision, filepath.Join(t.TempDir(), "github-output"))

	command := npmProfileSignCommand{
		preflight: func(context.Context, string, string) npmprofile.OIDCExchangeResult {
			return npmprofile.OIDCExchangeResult{WorkflowFilename: "release.yml"}
		},
		sign: func(context.Context, signing.Request) (signing.Result, error) {
			t.Fatal("signer called with mismatched built identity")
			return signing.Result{}, nil
		},
	}
	err := command.Execute(context.Background(), signSourceRefArguments(
		metadataDirectory,
		tarballDirectory,
		t.TempDir(),
		os.Getenv("GITHUB_OUTPUT"),
		digest.SumSHA256(metadataBytes).String(),
		digest.SumSHA256(tarball).String(),
		signBuiltRef,
		signOtherBuiltRevision,
	), &bytes.Buffer{})
	if err == nil {
		t.Fatal("Execute() accepted a built revision that differs from verified build metadata")
	}
	var identified interface{ DiagnosticID() string }
	if !errors.As(err, &identified) || identified.DiagnosticID() != npmprofile.IDReleaseRefMismatch {
		t.Fatalf("Execute() error = %v, want %s", err, npmprofile.IDReleaseRefMismatch)
	}
}

func TestNPMProfileSignCommandFallsBackToSingleInvocationIdentity(t *testing.T) {
	metadata, tarball := signSingleIdentityMetadata(t)
	metadataBytes := mustMarshalSignJSON(t, metadata)
	metadataDirectory := writeSignHandoff(t, "build-metadata.json", metadataBytes)
	tarballDirectory := writeSignHandoff(t, signTarballName, tarball)
	githubOutput := filepath.Join(t.TempDir(), "github-output")
	setSignGitHubEnvironment(t, signBuiltRef, signBuiltRevision, githubOutput)

	command := npmProfileSignCommand{
		preflight: func(context.Context, string, string) npmprofile.OIDCExchangeResult {
			return npmprofile.OIDCExchangeResult{WorkflowFilename: "release.yml"}
		},
		sign: func(ctx context.Context, request signing.Request) (signing.Result, error) {
			if err := ctx.Err(); err != nil {
				t.Fatal(err)
			}
			statement, err := provenance.DecodeStatement(request.Statement)
			if err != nil {
				t.Fatal(err)
			}
			parameters, err := npmprofile.DecodeExternalParameters(statement.Predicate.BuildDefinition.ExternalParameters)
			if err != nil {
				t.Fatal(err)
			}
			if parameters.Source.Ref != signBuiltRef || parameters.Source.Revision != signBuiltRevision ||
				parameters.Source.InputRef != nil || parameters.Source.InvocationRef != nil || parameters.Source.InvocationRevision != nil {
				t.Fatalf("single invocation source = %#v", parameters.Source)
			}
			return signing.Result{Statement: append([]byte(nil), request.Statement...), Bundle: []byte(`{"bundle":"fixture"}`)}, nil
		},
	}

	var output bytes.Buffer
	err := command.Execute(context.Background(), signSourceRefArguments(
		metadataDirectory,
		tarballDirectory,
		t.TempDir(),
		githubOutput,
		digest.SumSHA256(metadataBytes).String(),
		digest.SumSHA256(tarball).String(),
		"",
		"",
	), &output)
	if err != nil {
		t.Fatalf("Execute() error = %v, output = %s", err, output.String())
	}
}

func signSourceRefMetadata(t *testing.T) (npmprofile.BuildMetadata, []byte) {
	t.Helper()
	inputRef := signBuiltRef
	invocationRef := signInvocationRef
	invocationRevision := signInvocationRevision
	selectionManifest := "package.json"
	packageIdentityPreexisting := true
	packageVersionPreexisting := false
	builderID, err := identity.NewBuilderID(npmprofile.NPMWorkflowPath, signBuilderWorkflowSHA)
	if err != nil {
		t.Fatal(err)
	}
	parameters := npmprofile.ExternalParameters{
		Source: npmprofile.SourceParameters{
			Repository: "https://github.com/example/project", Ref: signBuiltRef, Revision: signBuiltRevision,
			EventName: "workflow_dispatch", RefType: "tag", InputRef: &inputRef,
			InvocationRef: &invocationRef, InvocationRevision: &invocationRevision,
		},
		Workflow: npmprofile.WorkflowParameters{Path: npmprofile.NPMWorkflowPath, SHA: signBuilderWorkflowSHA, BuilderID: builderID},
		Runtime:  npmprofile.RuntimeParameters{Runner: "ubuntu-24.04", NodeVersion: "24.0.0", NPMVersion: "11.5.1"},
		Package: npmprofile.PackageParameters{
			Directory: ".", SourceManifest: "package.json", Name: signPackageName, Version: "1.2.3",
			Repository: "https://github.com/example/project", TarballName: signTarballName,
			PackageURL: "https://registry.npmjs.org/%40windlass%2Fslsa-builder/1.2.3",
			PackedName: signPackageName, PackedVersion: "1.2.3",
		},
		PackageManager: npmprofile.PackageManagerParameters{
			Name: npmprofile.ManagerNPM, Version: "11.5.1", SelectionSource: npmprofile.SelectionPackageManager,
			SelectionManifest: &selectionManifest, SelectionManifestPath: &selectionManifest, Root: ".",
		},
		Publish: npmprofile.PublishParameters{
			ResolvedRegistryURL: signRegistryURL, ResolvedDistTag: "latest", EffectiveAccess: "existing-package-access",
			TrustedPublishing: true, ProvenanceFile: true, PackageIdentityPreexisting: &packageIdentityPreexisting,
			PackageVersionPreexisting: &packageVersionPreexisting,
		},
		Release:      npmprofile.ReleaseParameters{Ref: signBuiltRef, VersionTag: "v1.2.3"},
		Distribution: npmprofile.DistributionParameters{},
		Caller:       npmprofile.CallerParameters{WorkflowFilename: "release.yml"},
		Build:        npmprofile.BuildParameters{ScriptResult: npmprofile.BuildScriptSkippedAbsent},
	}
	externalParameters, err := npmprofile.EncodeExternalParameters(parameters)
	if err != nil {
		t.Fatal(err)
	}
	tarball := []byte("dispatch retry tarball")
	sha256Value := digest.SumSHA256(tarball).String()
	sha512Value := digest.SumSHA512(tarball).String()
	dependencies := []provenance.ResourceDescriptor{
		{
			Name: "lockfile", URI: "git+https://github.com/example/project@" + signBuiltRevision + "#package-lock.json",
			Digest: map[string]string{"sha256": sha256Value},
			Annotations: map[string]json.RawMessage{
				"package_manager":              mustMarshalSignJSON(t, npmprofile.ManagerNPM),
				"package_manager_root":         mustMarshalSignJSON(t, "."),
				"selection_source":             mustMarshalSignJSON(t, npmprofile.SelectionPackageManager),
				"selection_manifest_path":      mustMarshalSignJSON(t, "package.json"),
				"selection_lockfile_path":      mustMarshalSignJSON(t, "package-lock.json"),
				"stale_non_selected_lockfiles": mustMarshalSignJSON(t, []string{}),
			},
		},
		{
			Name: "runner-image", URI: "https://github.com/actions/runner-images/blob/main/images/ubuntu/Ubuntu2404-Readme.md",
			Annotations: map[string]json.RawMessage{
				"image_os":      mustMarshalSignJSON(t, "ubuntu24"),
				"image_version": mustMarshalSignJSON(t, "20260801.1.0"),
				"node_version":  mustMarshalSignJSON(t, "v24.0.0"),
			},
		},
	}
	return npmprofile.BuildMetadata{
		SchemaVersion: "1",
		PrimaryArtifact: npmprofile.PrimaryArtifact{
			ArtifactName: signTarballArtifactName, PayloadFileName: signTarballName,
			SHA256: sha256Value, SHA512: sha512Value,
		},
		ExternalParameters: externalParameters, ResolvedDependencies: dependencies,
	}, tarball
}

func signSingleIdentityMetadata(t *testing.T) (npmprofile.BuildMetadata, []byte) {
	t.Helper()
	metadata, tarball := signSourceRefMetadata(t)
	parameters, err := npmprofile.DecodeExternalParameters(metadata.ExternalParameters)
	if err != nil {
		t.Fatal(err)
	}
	parameters.Source.EventName = "push"
	parameters.Source.InputRef = nil
	parameters.Source.InvocationRef = nil
	parameters.Source.InvocationRevision = nil
	metadata.ExternalParameters, err = npmprofile.EncodeExternalParameters(parameters)
	if err != nil {
		t.Fatal(err)
	}
	return metadata, tarball
}

func signSourceRefArguments(metadataDirectory, tarballDirectory, outputDirectory, githubOutput, metadataSHA256, tarballSHA256, builtRef, builtRevision string) []string {
	arguments := []string{
		"--metadata-artifact-dir", metadataDirectory,
		"--metadata-sha256", metadataSHA256,
		"--metadata-artifact-name", signMetadataArtifactName,
		"--tarball-artifact-dir", tarballDirectory,
		"--tarball-sha256", tarballSHA256,
		"--tarball-artifact-name", signTarballArtifactName,
		"--node-version", "v24.0.0",
		"--registry-url", signRegistryURL,
		"--package-name", signPackageName,
		"--output-directory", outputDirectory,
		"--github-output", githubOutput,
	}
	if builtRef != "" || builtRevision != "" {
		arguments = append(arguments, "--built-ref", builtRef, "--built-revision", builtRevision)
	}
	return arguments
}

func setSignGitHubEnvironment(t *testing.T, ref, revision, githubOutput string) {
	t.Helper()
	values := map[string]string{
		"GITHUB_OUTPUT":              githubOutput,
		"GITHUB_REF":                 ref,
		"GITHUB_SHA":                 revision,
		"GITHUB_SERVER_URL":          "https://github.com",
		"GITHUB_REPOSITORY":          "example/project",
		"GITHUB_REPOSITORY_ID":       "1234",
		"GITHUB_REPOSITORY_OWNER_ID": "5678",
		"GITHUB_RUN_ID":              "123456789",
		"GITHUB_RUN_ATTEMPT":         "1",
		"WINDLASS_WORKFLOW_SHA":      signBuilderWorkflowSHA,
	}
	for name, value := range values {
		t.Setenv(name, value)
	}
}

func writeSignHandoff(t *testing.T, name string, contents []byte) string {
	t.Helper()
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, name), contents, 0o600); err != nil {
		t.Fatal(err)
	}
	return directory
}

func mustMarshalSignJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}
