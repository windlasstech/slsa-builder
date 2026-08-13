package command

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/windlasstech/slsa-builder/internal/npmprofile"
)

const (
	builtRevision      = "0123456789abcdef0123456789abcdef01234567"
	invocationRevision = "89abcdef0123456789abcdef0123456789abcdef"
)

func TestNPMProfileBuildCommandSourceRefDispatchRetry(t *testing.T) {
	repository := copyNPMBuildFixture(t)
	outputDirectory := emptyNPMBuildOutput(t)
	githubOutput := filepath.Join(t.TempDir(), "github-output")
	registry := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Errorf("registry method = %s", request.Method)
		}
		writer.Header().Set("Content-Type", "application/json")
		if _, err := writer.Write([]byte(`{"name":"windlass-fixture-unscoped","versions":{}}`)); err != nil {
			t.Errorf("write registry response: %v", err)
		}
	}))
	t.Cleanup(registry.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	var output bytes.Buffer
	err := npmProfileBuildCommand{httpClient: registry.Client(), runnerOverride: &npmprofile.RunnerCapture{
		ImageOS: "ubuntu24", ImageVersion: "20260801.1.0", IncludedSoftwareURL: "https://github.com/actions/runner-images/blob/main/images/ubuntu/Ubuntu2404-Readme.md",
	}}.Execute(ctx, npmBuildSourceRefArguments(repository, outputDirectory, githubOutput, registry.URL,
		"refs/tags/v1.0.0", "refs/heads/main", invocationRevision), &output)
	if err != nil {
		t.Fatal(err)
	}

	encodedOutputs, err := os.ReadFile(githubOutput)
	if err != nil {
		t.Fatal(err)
	}
	metadataBytes, err := os.ReadFile(parseGitHubOutputs(t, string(encodedOutputs))["build-metadata-path"])
	if err != nil {
		t.Fatal(err)
	}
	var metadata npmprofile.BuildMetadata
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatal(err)
	}
	parameters, err := npmprofile.DecodeExternalParameters(metadata.ExternalParameters)
	if err != nil {
		t.Fatal(err)
	}
	if parameters.Source.Ref != "refs/tags/v1.0.0" || parameters.Source.Revision != builtRevision || parameters.Source.RefType != "tag" {
		t.Fatalf("built source = %#v", parameters.Source)
	}
	if parameters.Source.InputRef == nil || *parameters.Source.InputRef != "refs/tags/v1.0.0" ||
		parameters.Source.InvocationRef == nil || *parameters.Source.InvocationRef != "refs/heads/main" ||
		parameters.Source.InvocationRevision == nil || *parameters.Source.InvocationRevision != invocationRevision {
		t.Fatalf("invocation source = %#v", parameters.Source)
	}
}

func TestNPMProfileBuildCommandRejectsSourceRefVersionBeforeBuild(t *testing.T) {
	repository := copyNPMBuildFixture(t)
	outputDirectory := emptyNPMBuildOutput(t)
	githubOutput := filepath.Join(t.TempDir(), "github-output")
	marker := filepath.Join(t.TempDir(), "package-manager-ran")
	t.Setenv("PATH", writeRejectingNPM(t, marker))
	transport := &countingRejectTransport{}

	dispatcher := NewDispatcher(npmProfileBuildCommand{httpClient: &http.Client{Transport: transport}})
	var output bytes.Buffer
	result := dispatcher.Dispatch(context.Background(), append([]string{"npm-profile-build"}, npmBuildSourceRefArguments(
		repository, outputDirectory, githubOutput, "http://127.0.0.1:1", "refs/tags/v1.0.1", "refs/heads/main", invocationRevision)...), &output)
	if result.ExitCode != ExitCodeVerificationFailure {
		t.Fatalf("exit code = %d, output = %s", result.ExitCode, output.String())
	}
	var report Report
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.PrimaryID == nil || *report.PrimaryID != npmprofile.IDSourceRefInvalid {
		t.Fatalf("primary ID = %v", report.PrimaryID)
	}
	if transport.requests != 0 {
		t.Fatalf("registry requests = %d, want 0", transport.requests)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("package-manager subprocess evidence: %v", err)
	}
}

type countingRejectTransport struct{ requests int }

func (transport *countingRejectTransport) RoundTrip(*http.Request) (*http.Response, error) {
	transport.requests++
	return nil, errors.New("registry access must not occur")
}

func copyNPMBuildFixture(t *testing.T) string {
	t.Helper()
	repository := filepath.Join(t.TempDir(), "repository")
	if err := os.CopyFS(repository, os.DirFS(filepath.Join("..", "..", "testdata", "npm", "packages", "npm-root-valid"))); err != nil {
		t.Fatal(err)
	}
	return repository
}

func emptyNPMBuildOutput(t *testing.T) string {
	t.Helper()
	outputDirectory := filepath.Join(t.TempDir(), "output")
	if err := os.Mkdir(outputDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	return outputDirectory
}

func npmBuildSourceRefArguments(repository, outputDirectory, githubOutput, registryURL, sourceRef, invocationRef, invocationSHA string) []string {
	return []string{
		"--repository-root", repository,
		"--package-directory", ".",
		"--observed-repository", "windlasstech/slsa-builder",
		"--output-directory", outputDirectory,
		"--artifact-name", "js-ts-npm-package-tarball-123456789-1",
		"--metadata-artifact-name", "js-ts-npm-build-metadata-123456789-1",
		"--registry-url", registryURL,
		"--event-name", "workflow_dispatch",
		"--ref-type", "tag",
		"--ref", sourceRef,
		"--revision", builtRevision,
		"--source-ref", sourceRef,
		"--invocation-ref", invocationRef,
		"--invocation-revision", invocationSHA,
		"--workflow-sha", builtRevision,
		"--caller-workflow-filename", "release.yml",
		"--github-output", githubOutput,
	}
}

func writeRejectingNPM(t *testing.T, marker string) string {
	t.Helper()
	directory := t.TempDir()
	script := "#!/bin/sh\n: > " + marker + "\nexit 97\n"
	if err := os.WriteFile(filepath.Join(directory, "npm"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return directory
}
