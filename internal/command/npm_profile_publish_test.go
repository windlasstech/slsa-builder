package command

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/diagnostic"
	"github.com/windlasstech/slsa-builder/internal/digest"
	"github.com/windlasstech/slsa-builder/internal/npmprofile"
)

func TestNPMProfilePublishCommandVerifiesHandoffsAndPersistsReport(t *testing.T) {
	tarball := []byte("tarball")
	bundle := []byte(`{"bundle":"fixture"}`)
	tarballDir := oneFileDirectory(t, "package.tgz", tarball)
	bundleDir := oneFileDirectory(t, "package.tgz.intoto.jsonl", bundle)
	reportPath := filepath.Join(t.TempDir(), "report.json")
	githubOutput := filepath.Join(t.TempDir(), "github-output")
	called := false
	command := npmProfilePublishCommand{publish: func(ctx context.Context, input npmProfilePublishInput) (npmprofile.PublishResult, error) {
		if err := ctx.Err(); err != nil {
			t.Fatal(err)
		}
		called = true
		if got, err := os.ReadFile(input.TarballPath); err != nil || !bytes.Equal(got, tarball) {
			t.Fatalf("verified tarball = %q, %v", got, err)
		}
		if got, err := os.ReadFile(input.BundlePath); err != nil || !bytes.Equal(got, bundle) {
			t.Fatalf("verified bundle = %q, %v", got, err)
		}
		report, err := diagnostic.Build(nil, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		return npmprofile.PublishResult{State: npmprofile.PublishCommittedAsExpected, Report: report}, nil
	}}
	err := command.Execute(context.Background(), publishArgs(t, tarballDir, bundleDir, reportPath, githubOutput, tarball, bundle), &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("publish convergence was not invoked")
	}
	if _, err := ReadTypedJSON[diagnostic.Report](reportPath, nil); err != nil {
		t.Fatalf("persistent report is invalid: %v", err)
	}
	outputs := parseGitHubOutputs(t, string(mustRead(t, githubOutput)))
	if outputs["package-name"] != "@windlass/test" || outputs["package-version"] != "1.2.3" || outputs["result"] != "pass" {
		t.Fatalf("publish outputs = %#v", outputs)
	}
}

func TestNPMProfilePublishCommandRejectsAmbiguousHandoff(t *testing.T) {
	tarball := []byte("tarball")
	bundle := []byte(`{"bundle":"fixture"}`)
	tarballDir := oneFileDirectory(t, "package.tgz", tarball)
	if err := os.WriteFile(filepath.Join(tarballDir, "extra"), []byte("extra"), 0o600); err != nil {
		t.Fatal(err)
	}
	bundleDir := oneFileDirectory(t, "package.tgz.intoto.jsonl", bundle)
	command := npmProfilePublishCommand{publish: func(context.Context, npmProfilePublishInput) (npmprofile.PublishResult, error) {
		t.Fatal("publish called after ambiguous handoff")
		return npmprofile.PublishResult{}, nil
	}}
	if err := command.Execute(context.Background(), publishArgs(t, tarballDir, bundleDir, filepath.Join(t.TempDir(), "report.json"), filepath.Join(t.TempDir(), "out"), tarball, bundle), &bytes.Buffer{}); err == nil {
		t.Fatal("Execute() accepted a multi-file handoff")
	}
}

func TestNPMProfilePublishCommandPersistsOIDCPreflightFailure(t *testing.T) {
	tarball := []byte("tarball")
	bundle := []byte(`{"bundle":"fixture"}`)
	reportPath := filepath.Join(t.TempDir(), "report.json")
	for _, name := range []string{"ACTIONS_ID_TOKEN_REQUEST_URL", "ACTIONS_ID_TOKEN_REQUEST_TOKEN", "GITHUB_WORKFLOW_REF"} {
		t.Setenv(name, "")
	}
	err := NewNPMProfilePublishCommand().Execute(context.Background(), publishArgs(t,
		oneFileDirectory(t, "package.tgz", tarball), oneFileDirectory(t, "package.tgz.intoto.jsonl", bundle),
		reportPath, filepath.Join(t.TempDir(), "out"), tarball, bundle), &bytes.Buffer{})
	if !errors.Is(err, ErrVerificationFailure) {
		t.Fatalf("Execute() error = %v, want verification failure", err)
	}
	report, readErr := ReadTypedJSON[diagnostic.Report](reportPath, nil)
	if readErr != nil || report.PrimaryID == nil || *report.PrimaryID != npmprofile.IDOIDCCapabilityUnavailable {
		t.Fatalf("preflight report = %#v, %v", report, readErr)
	}
}

func TestNPMProfileReportCommand(t *testing.T) {
	report, err := diagnostic.Build(nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := report.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(t.TempDir(), "report.json")
	if err := os.WriteFile(reportPath, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	githubOutput := filepath.Join(t.TempDir(), "out")
	if err := NewNPMProfileReportCommand().Execute(context.Background(), []string{"--report-path", reportPath, "--github-output", githubOutput}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if got := parseGitHubOutputs(t, string(mustRead(t, githubOutput)))["result"]; got != "pass" {
		t.Fatalf("result = %q", got)
	}
}

func publishArgs(t *testing.T, tarballDir, bundleDir, reportPath, githubOutput string, tarball, bundle []byte) []string {
	t.Helper()
	return []string{
		"--tarball-artifact-dir", tarballDir, "--tarball-artifact-name", "tarball-artifact", "--tarball-name", "package.tgz",
		"--tarball-sha256", digest.SumSHA256(tarball).String(), "--tarball-sha512", digest.SumSHA512(tarball).String(),
		"--bundle-artifact-dir", bundleDir, "--bundle-artifact-name", "bundle-artifact", "--bundle-name", "package.tgz.intoto.jsonl", "--bundle-sha256", digest.SumSHA256(bundle).String(),
		"--package-name", "@windlass/test", "--package-version", "1.2.3", "--package-url", "https://registry.npmjs.org/%40windlass%2Ftest/1.2.3",
		"--registry-url", "https://registry.npmjs.org/", "--npm-executable", "/usr/local/bin/npm", "--report-path", reportPath, "--github-output", githubOutput,
	}
}

func oneFileDirectory(t *testing.T, name string, content []byte) string {
	t.Helper()
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, name), content, 0o600); err != nil {
		t.Fatal(err)
	}
	return directory
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	value, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return value
}
