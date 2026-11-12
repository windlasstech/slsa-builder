package command

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/digest"
	"github.com/windlasstech/slsa-builder/internal/npmprofile"
)

func TestVerifyAttestationRequiresManifestEvidence(t *testing.T) {
	t.Parallel()
	paths := writeSourceBindingInputs(t, sourceBindingPolicy("https://github.com/example/project", "refs/tags/v1.2.3", commandBuiltSHA), sourceBindingIdentity("https://github.com/example/project", "1", "2", "refs/heads/main", commandInvocationSHA), sourceBindingStatement(t, sourceBindingStatementOptions{}))
	args := append(paths.args(), "--manifest-evidence", filepath.Join(t.TempDir(), "missing.json"))
	var output bytes.Buffer
	result := NewDispatcher(NewVerifyAttestationCommand()).Dispatch(t.Context(), args, &output)
	requireSourceBindingDiagnostic(t, output.Bytes(), result.ExitCode, npmprofile.IDPackageManifestEvidence)
}

func TestPublishCommandClassifiesManifestEvidenceMismatch(t *testing.T) {
	t.Parallel()
	_, tarball := signSingleIdentityMetadata(t)
	bundle := []byte("bundle")
	args := publishArgs(t, oneFileDirectory(t, "package.tgz", tarball), oneFileDirectory(t, "package.tgz.intoto.jsonl", bundle), filepath.Join(t.TempDir(), "report.json"), filepath.Join(t.TempDir(), "out"), tarball, bundle)
	var directory string
	for index, value := range args {
		if value == "--metadata-artifact-dir" {
			directory = args[index+1]
		}
	}
	filename := filepath.Join(directory, "build-metadata.json")
	var metadata npmprofile.BuildMetadata
	if err := json.Unmarshal(mustRead(t, filename), &metadata); err != nil {
		t.Fatal(err)
	}
	metadata.ManifestEvidence.ContentBase64 = "e30="
	encoded := mustMarshalSignJSON(t, metadata)
	if err := os.WriteFile(filename, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	for index, value := range args {
		if value == "--metadata-sha256" {
			args[index+1] = digest.SumSHA256(encoded).String()
		}
	}
	command := npmProfilePublishCommand{publish: func(context.Context, npmProfilePublishInput) (npmprofile.PublishResult, error) {
		t.Fatal("publish called with mismatched evidence")
		return npmprofile.PublishResult{}, nil
	}}
	var output bytes.Buffer
	result := NewDispatcher(command).Dispatch(t.Context(), append([]string{command.Name()}, args...), &output)
	requireSourceBindingDiagnostic(t, output.Bytes(), result.ExitCode, npmprofile.IDPackageManifestEvidence)
}
