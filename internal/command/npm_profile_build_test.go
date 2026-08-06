package command

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	err := NewNPMProfileBuildCommand().Execute(ctx, []string{
		"--repository-root", repository,
		"--package-directory", ".",
		"--observed-repository", "windlasstech/slsa-builder",
		"--output-directory", outputDirectory,
		"--artifact-name", artifactName,
		"--metadata-artifact-name", metadataArtifactName,
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
