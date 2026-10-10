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
	"strings"
	"testing"
	"time"

	"github.com/windlasstech/slsa-builder/internal/npmprofile"
)

func TestNPMProfileBuildCommand(t *testing.T) {
	repository := filepath.Join(t.TempDir(), "repository")
	source := filepath.Join("..", "..", "testdata", "npm", "packages", "npm-root-valid")
	if err := os.CopyFS(repository, os.DirFS(source)); err != nil {
		t.Fatal(err)
	}
	outputDirectory := filepath.Join(t.TempDir(), "output")
	if err := os.Mkdir(outputDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	githubOutput := filepath.Join(t.TempDir(), "github-output")
	artifactName := "js-ts-npm-package-tarball-123456789-1"
	metadataArtifactName := "js-ts-npm-build-metadata-123456789-1"
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)

	var output bytes.Buffer
	registry := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Fatalf("registry method = %s", request.Method)
		}
		writer.Header().Set("Content-Type", "application/json")
		if _, err := writer.Write([]byte(`{"name":"windlass-fixture-unscoped","versions":{}}`)); err != nil {
			t.Errorf("write registry response: %v", err)
		}
	}))
	t.Cleanup(registry.Close)
	err := npmProfileBuildCommand{httpClient: registry.Client(), runnerOverride: &npmprofile.RunnerCapture{
		ImageOS: "ubuntu24", ImageVersion: "20260801.1.0", IncludedSoftwareURL: "https://github.com/actions/runner-images/blob/main/images/ubuntu/Ubuntu2404-Readme.md",
	}}.Execute(ctx, []string{
		"--repository-root", repository,
		"--package-directory", ".",
		"--observed-repository", "windlasstech/slsa-builder",
		"--output-directory", outputDirectory,
		"--artifact-name", artifactName,
		"--metadata-artifact-name", metadataArtifactName,
		"--registry-url", registry.URL,
		"--event-name", "push",
		"--ref-type", "tag",
		"--ref", "refs/tags/v1.0.0",
		"--revision", "0123456789abcdef0123456789abcdef01234567",
		"--workflow-sha", "0123456789abcdef0123456789abcdef01234567",
		"--caller-workflow-filename", "release.yml",
		"--github-output", githubOutput,
	}, &output)
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := os.ReadFile(githubOutput)
	if err != nil {
		t.Fatal(err)
	}
	outputs := parseGitHubOutputs(t, string(encoded))
	if outputs["tarball-artifact-name"] != artifactName {
		t.Fatalf("tarball artifact name = %q", outputs["tarball-artifact-name"])
	}
	if outputs["build-metadata-artifact-name"] != metadataArtifactName {
		t.Fatalf("metadata artifact name = %q", outputs["build-metadata-artifact-name"])
	}
	if filepath.Base(outputs["build-metadata-path"]) != "build-metadata.json" || len(outputs["build-metadata-sha256"]) != 64 {
		t.Fatalf("metadata outputs = %#v", outputs)
	}
	if !strings.HasSuffix(outputs["tarball-name"], ".tgz") || len(outputs["tarball-sha256"]) != 64 || len(outputs["tarball-sha512"]) != 128 {
		t.Fatalf("tarball outputs = %#v", outputs)
	}
}

func TestNPMProfileBuildCommandRejectsMissingHandoffName(t *testing.T) {
	var output bytes.Buffer
	if err := NewNPMProfileBuildCommand().Execute(context.Background(), nil, &output); err == nil {
		t.Fatal("Execute() succeeded, want required argument error")
	}
}

// digestMismatchFailure stands in for the npmprofile ADR 0092 classified
// error: any error exposing DiagnosticID() == package-manager-digest-mismatch
// must map to a fail report plus ErrVerificationFailure (exit 1).
type digestMismatchFailure struct{ message string }

func (err digestMismatchFailure) Error() string { return err.message }

func (digestMismatchFailure) DiagnosticID() string { return npmprofile.IDPackageManagerDigestMismatch }

func TestWriteNPMBuildDigestMismatch(t *testing.T) {
	t.Run("classified digest mismatch emits a fail report", func(t *testing.T) {
		var output bytes.Buffer
		classified, err := writeNPMBuildDigestMismatch(&output, digestMismatchFailure{message: "pnpm distribution sha512 digest disagrees with the declared descriptor digest"})
		if err != nil {
			t.Fatal(err)
		}
		if !classified {
			t.Fatal("classified digest mismatch was not recognized")
		}
		var report Report
		if err := json.Unmarshal(output.Bytes(), &report); err != nil {
			t.Fatalf("decode report: %v", err)
		}
		if report.Result != "fail" || report.ExitCode != ExitCodeVerificationFailure {
			t.Fatalf("report = %#v, want fail with exit code 1", report)
		}
		if report.PrimaryID == nil || *report.PrimaryID != npmprofile.IDPackageManagerDigestMismatch {
			t.Fatalf("primary ID = %#v, want %s", report.PrimaryID, npmprofile.IDPackageManagerDigestMismatch)
		}
	})
	t.Run("unclassified errors pass through", func(t *testing.T) {
		var output bytes.Buffer
		classified, err := writeNPMBuildDigestMismatch(&output, errors.New("corepack did not report the acquired distribution URL"))
		if err != nil || classified {
			t.Fatalf("classified = %t, err = %v, want pass-through", classified, err)
		}
		if output.Len() != 0 {
			t.Fatalf("output = %q, want no report for an unclassified error", output.String())
		}
	})
}

func parseGitHubOutputs(t *testing.T, encoded string) map[string]string {
	t.Helper()
	outputs := make(map[string]string)
	for line := range strings.Lines(encoded) {
		name, value, ok := strings.Cut(strings.TrimSuffix(line, "\n"), "=")
		if !ok {
			t.Fatalf("malformed GitHub output line %q", line)
		}
		outputs[name] = value
	}
	return outputs
}
