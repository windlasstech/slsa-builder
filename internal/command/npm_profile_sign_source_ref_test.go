package command

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/digest"
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
			if got := statement.Predicate.RunDetails.Metadata; got.StartedOn != metadata.StartedOn || got.FinishedOn != metadata.FinishedOn {
				t.Fatalf("signed build timestamps = %#v", got)
			}
			parameters, err := npmprofile.DecodeExternalParameters(statement.Predicate.BuildDefinition.ExternalParameters)
			if err != nil {
				t.Fatalf("decode signed external parameters: %v", err)
			}
			if parameters.Source.Ref != signBuiltRef || parameters.Workflow.Ref != signInvocationRef {
				t.Fatalf("signed selectors = %#v", parameters)
			}
			for _, descriptor := range statement.Predicate.BuildDefinition.ResolvedDependencies {
				if descriptor.Name == "built-source" && (descriptor.URI != "git+https://github.com/example/project@"+signBuiltRef || descriptor.Digest["gitCommit"] != signBuiltRevision) {
					t.Fatalf("built source = %#v", descriptor)
				}
				if descriptor.Name == "invocation-source" && (descriptor.URI != "git+https://github.com/example/project@"+signInvocationRef || descriptor.Digest["gitCommit"] != signInvocationRevision) {
					t.Fatalf("invocation source = %#v", descriptor)
				}
			}
			if request.Identity.SourceRef != signInvocationRef || request.Identity.SourceDigest != signInvocationRevision {
				t.Fatalf("certificate context = %#v", request.Identity)
			}
			return signing.Result{Statement: append([]byte(nil), request.Statement...), Bundle: []byte(`{"bundle":"fixture"}`)}, nil
		},
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
	var output bytes.Buffer
	err := command.Execute(context.Background(), signSourceRefArguments(
		metadataDirectory,
		tarballDirectory,
		t.TempDir(),
		os.Getenv("GITHUB_OUTPUT"),
		digest.SumSHA256(metadataBytes).String(),
		digest.SumSHA256(tarball).String(),
		signBuiltRef,
		signOtherBuiltRevision,
	), &output)
	if !errors.Is(err, ErrVerificationFailure) {
		t.Fatalf("Execute() error = %v, want ErrVerificationFailure", err)
	}
	assertPrimaryDiagnostic(t, output.Bytes(), ExitCodeVerificationFailure, npmprofile.IDBuiltSource)
}

func TestNPMProfileSignDispatcherClassifiesMismatchedBuiltIdentityAsPolicyFailure(t *testing.T) {
	metadata, tarball := signSourceRefMetadata(t)
	metadataBytes := mustMarshalSignJSON(t, metadata)
	metadataDirectory := writeSignHandoff(t, "build-metadata.json", metadataBytes)
	tarballDirectory := writeSignHandoff(t, signTarballName, tarball)
	githubOutput := filepath.Join(t.TempDir(), "github-output")
	setSignGitHubEnvironment(t, signInvocationRef, signInvocationRevision, githubOutput)

	command := npmProfileSignCommand{
		preflight: func(context.Context, string, string) npmprofile.OIDCExchangeResult {
			return npmprofile.OIDCExchangeResult{WorkflowFilename: "release.yml"}
		},
		sign: func(context.Context, signing.Request) (signing.Result, error) {
			t.Fatal("signer called with mismatched built identity")
			return signing.Result{}, nil
		},
	}
	arguments := append([]string{command.Name()}, signSourceRefArguments(
		metadataDirectory,
		tarballDirectory,
		t.TempDir(),
		githubOutput,
		digest.SumSHA256(metadataBytes).String(),
		digest.SumSHA256(tarball).String(),
		signBuiltRef,
		signOtherBuiltRevision,
	)...)

	var output bytes.Buffer
	result := NewDispatcher(command).Dispatch(context.Background(), arguments, &output)
	if result.ExitCode != ExitCodeVerificationFailure {
		t.Fatalf("dispatch exit code = %d, want %d", result.ExitCode, ExitCodeVerificationFailure)
	}
	assertPrimaryDiagnostic(t, output.Bytes(), ExitCodeVerificationFailure, npmprofile.IDBuiltSource)
}

func TestNPMProfileSignCommandFallsBackToSingleInvocationIdentity(t *testing.T) {
	metadata, tarball := signSingleIdentityMetadata(t)
	metadataBytes := mustMarshalSignJSON(t, metadata)
	metadataDirectory := writeSignHandoff(t, "build-metadata.json", metadataBytes)
	tarballDirectory := writeSignHandoff(t, signTarballName, tarball)
	githubOutput := filepath.Join(t.TempDir(), "github-output")
	setSignGitHubEnvironment(t, signBuiltRef, signBuiltRevision, githubOutput)
	t.Setenv("GITHUB_EVENT_NAME", "push")

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
			if parameters.Source.Ref != "" || parameters.Workflow.Ref != signBuiltRef {
				t.Fatalf("single invocation source = %#v", parameters.Source)
			}
			for _, descriptor := range statement.Predicate.BuildDefinition.ResolvedDependencies {
				if descriptor.Name == "invocation-source" || descriptor.Name == "built-source" {
					if descriptor.URI != "git+https://github.com/example/project@"+signBuiltRef || descriptor.Digest["gitCommit"] != signBuiltRevision {
						t.Fatalf("single source role = %#v", descriptor)
					}
				}
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
	parameters := npmprofile.ExternalParameters{
		Source:   npmprofile.SourceParameters{Ref: signBuiltRef},
		Workflow: npmprofile.WorkflowParameters{Ref: signInvocationRef, Repository: "https://github.com/example/project", Path: ".github/workflows/release.yml"},
		Package:  npmprofile.PackageParameters{Directory: "."},
	}
	externalParameters, err := npmprofile.EncodeExternalParameters(parameters)
	if err != nil {
		t.Fatal(err)
	}
	manifest := []byte(`{"name":"@windlass/slsa-builder","version":"1.2.3","repository":"example/project"}`)
	var buffer bytes.Buffer
	compressed := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(compressed)
	if err := archive.WriteHeader(&tar.Header{Name: "package/package.json", Mode: 0o644, Size: int64(len(manifest))}); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Write(manifest); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	tarball := buffer.Bytes()
	sha256Value := digest.SumSHA256(tarball).String()
	sha512Value := digest.SumSHA512(tarball).String()
	dependencies := []provenance.ResourceDescriptor{
		{Name: "invocation-source", URI: "git+https://github.com/example/project@" + signInvocationRef, Digest: map[string]string{"gitCommit": signInvocationRevision}},
		{Name: "built-source", URI: "git+https://github.com/example/project@" + signBuiltRef, Digest: map[string]string{"gitCommit": signBuiltRevision}, Annotations: map[string]json.RawMessage{"ref_type": json.RawMessage(`"tag"`)}},
		{Name: "package-manifest", URI: "git+https://github.com/example/project@" + signBuiltRevision + "#package.json", Digest: map[string]string{"sha256": digest.SumSHA256(manifest).String()}, MediaType: "application/json", Annotations: map[string]json.RawMessage{"workspace_root": json.RawMessage(`null`)}},
		{
			Name: "lockfile", URI: "git+https://github.com/example/project@" + signBuiltRevision + "#package-lock.json",
			MediaType: "application/json",
			Digest:    map[string]string{"sha256": sha256Value},
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
			Name:      "node-runtime",
			MediaType: "application/gzip",
			URI:       "https://nodejs.org/download/release/v24.22.1/node-v24.22.1-linux-x64.tar.gz",
			Digest:    map[string]string{"sha256": "eb6728e77c576a35162c122c45ccadf224d89ff00abc8dae46cc2897c6002d7c"},
			Annotations: map[string]json.RawMessage{
				"digest_authority": mustMarshalSignJSON(t, "builder-pinned-record"),
				"platform":         mustMarshalSignJSON(t, "linux-x64"),
			},
		},
		{
			Name: "runner-image", URI: "https://github.com/actions/runner-images/blob/main/images/ubuntu/Ubuntu2404-Readme.md",
			Annotations: map[string]json.RawMessage{
				"deployment_version": mustMarshalSignJSON(t, "20260801.1.1"),
				"image_os":           mustMarshalSignJSON(t, "ubuntu24"),
				"image_version":      mustMarshalSignJSON(t, "20260801.1.0"),
			},
		},
	}
	return npmprofile.BuildMetadata{
		SchemaVersion: "1",
		StartedOn:     "2026-08-14T11:59:58Z", FinishedOn: "2026-08-14T12:00:03Z",
		PrimaryArtifact: npmprofile.PrimaryArtifact{
			ArtifactName: signTarballArtifactName, PayloadFileName: signTarballName,
			SHA256: sha256Value, SHA512: sha512Value,
		},
		ExternalParameters: externalParameters, ResolvedDependencies: dependencies,
		ManifestEvidence:       npmprofile.ManifestEvidence{Path: "package.json", ContentBase64: base64.StdEncoding.EncodeToString(manifest)},
		BuildScriptDisposition: npmprofile.BuildScriptSkippedAbsent,
	}, tarball
}

func TestNPMProfileSignRecordsStatementHashOnSignFailure(t *testing.T) {
	metadata, tarball := signSingleIdentityMetadata(t)
	metadataBytes := mustMarshalSignJSON(t, metadata)
	metadataDirectory := writeSignHandoff(t, "build-metadata.json", metadataBytes)
	tarballDirectory := writeSignHandoff(t, signTarballName, tarball)
	githubOutput := filepath.Join(t.TempDir(), "github-output")
	setSignGitHubEnvironment(t, signBuiltRef, signBuiltRevision, githubOutput)
	t.Setenv("GITHUB_EVENT_NAME", "push")

	var signedStatement []byte
	command := npmProfileSignCommand{
		preflight: func(context.Context, string, string) npmprofile.OIDCExchangeResult {
			return npmprofile.OIDCExchangeResult{WorkflowFilename: "release.yml"}
		},
		sign: func(ctx context.Context, request signing.Request) (signing.Result, error) {
			if err := ctx.Err(); err != nil {
				return signing.Result{}, err
			}
			signedStatement = append([]byte(nil), request.Statement...)
			return signing.Result{}, errors.New("post-sign offline verification: injected failure")
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
	if err == nil {
		t.Fatal("Execute() succeeded with a failing signer")
	}
	if len(signedStatement) == 0 {
		t.Fatal("failing signer never received the assembled Statement")
	}
	want := "statement-sha256=" + digest.SumSHA256(signedStatement).String()
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("failure error does not record the assembled Statement digest %q: %v", want, err)
	}
}

func signSingleIdentityMetadata(t *testing.T) (npmprofile.BuildMetadata, []byte) {
	t.Helper()
	metadata, tarball := signSourceRefMetadata(t)
	parameters, err := npmprofile.DecodeExternalParameters(metadata.ExternalParameters)
	if err != nil {
		t.Fatal(err)
	}
	parameters.Workflow.Ref = parameters.Source.Ref
	parameters.Source = npmprofile.SourceParameters{}
	for index := range metadata.ResolvedDependencies {
		if metadata.ResolvedDependencies[index].Name == "invocation-source" {
			metadata.ResolvedDependencies[index].URI = "git+https://github.com/example/project@" + signBuiltRef
			metadata.ResolvedDependencies[index].Digest["gitCommit"] = signBuiltRevision
		}
	}
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
		"--node-version", "v24.22.1",
		"--npm-version", "11.19.1",
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
		"GITHUB_EVENT_NAME":          "workflow_dispatch",
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
