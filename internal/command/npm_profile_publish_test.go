package command

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
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
	_, tarball := signSingleIdentityMetadata(t)
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
	if outputs["package-name"] != signPackageName || outputs["package-version"] != "1.2.3" || outputs["result"] != "pass" {
		t.Fatalf("publish outputs = %#v", outputs)
	}
}

func TestNPMProfilePublishCommandRejectsAmbiguousHandoff(t *testing.T) {
	_, tarball := signSingleIdentityMetadata(t)
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
	_, tarball := signSingleIdentityMetadata(t)
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

func TestNPMProfilePublishInputClassification(t *testing.T) {
	_, tarball := signSingleIdentityMetadata(t)
	bundle := []byte(`{"bundle":"fixture"}`)

	t.Run("missing metadata flags are invocation errors without a report", func(t *testing.T) {
		reportPath := filepath.Join(t.TempDir(), "report.json")
		args := publishArgs(t, oneFileDirectory(t, "package.tgz", tarball), oneFileDirectory(t, "package.tgz.intoto.jsonl", bundle),
			reportPath, filepath.Join(t.TempDir(), "out"), tarball, bundle)
		filtered := args[:0]
		for index := 0; index < len(args); index += 2 {
			if args[index] == "--metadata-artifact-dir" || args[index] == "--metadata-artifact-name" || args[index] == "--metadata-sha256" {
				continue
			}
			filtered = append(filtered, args[index], args[index+1])
		}
		err := NewNPMProfilePublishCommand().Execute(context.Background(), filtered, &bytes.Buffer{})
		if err == nil || err.Error() != "all npm-profile-publish flags are required with no positional arguments" || errors.Is(err, ErrVerificationFailure) {
			t.Fatalf("Execute() error = %v, want the plain required-flags invocation error", err)
		}
		if _, statErr := os.Stat(reportPath); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("invocation failure wrote a report: %v", statErr)
		}
	})

	t.Run("publish arguments diverging from authenticated metadata persist a verification report", func(t *testing.T) {
		reportPath := filepath.Join(t.TempDir(), "report.json")
		args := publishArgs(t, oneFileDirectory(t, "package.tgz", tarball), oneFileDirectory(t, "package.tgz.intoto.jsonl", bundle),
			reportPath, filepath.Join(t.TempDir(), "out"), tarball, bundle)
		for index, arg := range args {
			if arg == "--package-name" {
				args[index+1] = "@windlass/other"
			}
		}
		err := NewNPMProfilePublishCommand().Execute(context.Background(), args, &bytes.Buffer{})
		if !errors.Is(err, ErrVerificationFailure) {
			t.Fatalf("Execute() error = %v, want verification failure", err)
		}
		report, readErr := ReadTypedJSON[diagnostic.Report](reportPath, nil)
		if readErr != nil || report.PrimaryID == nil || *report.PrimaryID != npmprofile.IDPackageManifestEvidence {
			t.Fatalf("mismatch report = %#v, %v", report, readErr)
		}
	})

	t.Run("handoff record digests disagreeing with the authenticated metadata keep their diagnostic in the report", func(t *testing.T) {
		other := buildPublishTestTarball(t, []byte(`{"name":"@windlass/slsa-builder","version":"1.2.4","repository":"example/project"}`))
		reportPath := filepath.Join(t.TempDir(), "report.json")
		tarballDir := oneFileDirectory(t, "package.tgz", other)
		args := publishArgs(t, tarballDir, oneFileDirectory(t, "package.tgz.intoto.jsonl", bundle),
			reportPath, filepath.Join(t.TempDir(), "out"), tarball, bundle)
		// The on-disk bytes and the handoff record agree with each other but
		// not with the authenticated metadata, so the identity binding rejects
		// before packed-artifact verification runs. Packed-artifact digest
		// verification itself is covered directly in the npmprofile tests.
		for index, arg := range args {
			switch arg {
			case "--tarball-sha256":
				args[index+1] = digest.SumSHA256(other).String()
			case "--tarball-sha512":
				args[index+1] = digest.SumSHA512(other).String()
			}
		}
		err := NewNPMProfilePublishCommand().Execute(context.Background(), args, &bytes.Buffer{})
		if !errors.Is(err, ErrVerificationFailure) {
			t.Fatalf("Execute() error = %v, want verification failure", err)
		}
		report, readErr := ReadTypedJSON[diagnostic.Report](reportPath, nil)
		if readErr != nil || report.PrimaryID == nil || *report.PrimaryID != "windlass.verify.error.digest-mismatch" {
			t.Fatalf("digest mismatch report = %#v, %v", report, readErr)
		}
	})

	t.Run("truncated packed artifact reports the pack failure diagnostic", func(t *testing.T) {
		truncated := tarball[:len(tarball)-16]
		reportPath := filepath.Join(t.TempDir(), "report.json")
		tarballDir := oneFileDirectory(t, "package.tgz", truncated)
		args := publishArgs(t, tarballDir, oneFileDirectory(t, "package.tgz.intoto.jsonl", bundle),
			reportPath, filepath.Join(t.TempDir(), "out"), truncated, bundle)
		err := NewNPMProfilePublishCommand().Execute(context.Background(), args, &bytes.Buffer{})
		if !errors.Is(err, ErrVerificationFailure) {
			t.Fatalf("Execute() error = %v, want verification failure", err)
		}
		report, readErr := ReadTypedJSON[diagnostic.Report](reportPath, nil)
		if readErr != nil || report.PrimaryID == nil || *report.PrimaryID != "windlass.verify.error.package-pack-failed" {
			t.Fatalf("pack failure report = %#v, %v", report, readErr)
		}
	})
}

func buildPublishTestTarball(t *testing.T, manifest []byte) []byte {
	t.Helper()
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
	return buffer.Bytes()
}

func publishArgs(t *testing.T, tarballDir, bundleDir, reportPath, githubOutput string, tarball, bundle []byte) []string {
	t.Helper()
	metadata, _ := signSingleIdentityMetadata(t)
	metadata.PrimaryArtifact = npmprofile.PrimaryArtifact{ArtifactName: "tarball-artifact", PayloadFileName: "package.tgz", SHA256: digest.SumSHA256(tarball).String(), SHA512: digest.SumSHA512(tarball).String()}
	encoded := mustMarshalSignJSON(t, metadata)
	metadataDir := oneFileDirectory(t, "build-metadata.json", encoded)
	return []string{
		"--metadata-artifact-dir", metadataDir, "--metadata-artifact-name", "metadata-artifact", "--metadata-sha256", digest.SumSHA256(encoded).String(),
		"--tarball-artifact-dir", tarballDir, "--tarball-artifact-name", "tarball-artifact", "--tarball-name", "package.tgz",
		"--tarball-sha256", digest.SumSHA256(tarball).String(), "--tarball-sha512", digest.SumSHA512(tarball).String(),
		"--bundle-artifact-dir", bundleDir, "--bundle-artifact-name", "bundle-artifact", "--bundle-name", "package.tgz.intoto.jsonl", "--bundle-sha256", digest.SumSHA256(bundle).String(),
		"--package-name", signPackageName, "--package-version", "1.2.3", "--package-url", "https://registry.npmjs.org/%40windlass%2Fslsa-builder/1.2.3",
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
