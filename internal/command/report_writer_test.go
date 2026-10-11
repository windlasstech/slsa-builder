package command

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/diagnostic"
	"github.com/windlasstech/slsa-builder/internal/digest"
	"github.com/windlasstech/slsa-builder/internal/handoff"
	"github.com/windlasstech/slsa-builder/internal/npmprofile"
)

func TestReportHelpersPropagateSinkFailures(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name  string
		write func(io.Writer) error
	}{
		{"invocation", func(out io.Writer) error { return writeInvocationReport(out, "invocation failed") }},
		{"diagnostic", func(out io.Writer) error {
			return writeDiagnosticError(out, diagnostic.IDInputUnavailable, "input", "missing input")
		}},
		{"build policy", func(out io.Writer) error {
			return writeNPMBuildPolicyError(out, newIdentityPolicyBindingError(npmprofile.IDSourceRefInvalid, "invalid source"))
		}},
		{"sign policy", func(out io.Writer) error {
			return writeNPMProfileSignPolicyError(out, newIdentityPolicyBindingError(npmprofile.IDReleaseRefMismatch, "invalid release"))
		}},
		{"digest mismatch", func(out io.Writer) error {
			classified, err := writeNPMBuildDigestMismatch(out, digestMismatchFailure{message: "digest mismatch"})
			if classified {
				t.Error("failed report write classified as emitted")
			}
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			cause := errors.New("sink failed")
			err := test.write(&stagedWriter{err: cause})
			if !errors.Is(err, cause) || errors.Is(err, ErrVerificationFailure) || errors.Is(err, ErrInvocationFailure) {
				t.Fatalf("writer error = %v, want original sink failure without report sentinel", err)
			}
		})
	}
}

func TestReportHelpersRejectInvalidDiagnostics(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name  string
		write func(io.Writer) error
	}{
		{"unknown diagnostic", func(out io.Writer) error { return writeDiagnosticError(out, "unknown", "test", "message") }},
		{"invalid entry", func(out io.Writer) error { return writeDiagnostics(out, nil, []diagnostic.Diagnostic{{ID: "unknown"}}) }},
		{"unknown sign policy", func(out io.Writer) error {
			return writeNPMProfileSignPolicyError(out, newIdentityPolicyBindingError("unknown", "message"))
		}},
		{"empty invocation message", func(out io.Writer) error { return writeInvocationReport(out, "") }},
		{"empty source message", func(out io.Writer) error {
			return writeSourceRefFailure(out, newIdentityPolicyBindingError(npmprofile.IDSourceRefInvalid, ""))
		}},
		{"empty build message", func(out io.Writer) error {
			return writeNPMBuildPolicyError(out, newIdentityPolicyBindingError(npmprofile.IDSourceRefInvalid, ""))
		}},
		{"empty digest message", func(out io.Writer) error {
			_, err := writeNPMBuildDigestMismatch(out, digestMismatchFailure{})
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var output bytes.Buffer
			err := test.write(&output)
			var contract *diagnostic.ContractError
			if !errors.As(err, &contract) || contract.ID != diagnosticsContractInvalidID || output.Len() != 0 {
				t.Fatalf("invalid diagnostic error = %v, want contract error without output; output = %q", err, output.String())
			}
		})
	}
}

func TestPolicyReportHelpersPreserveUnclassifiedCauses(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name  string
		write func(io.Writer, error) error
	}{
		{"build", writeNPMBuildPolicyError},
		{"sign", writeNPMProfileSignPolicyError},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			cause := errors.New("unclassified failure")
			var output bytes.Buffer
			if err := test.write(&output, cause); err != cause || output.Len() != 0 {
				t.Fatalf("error = %v, output = %q, want unchanged cause without report", err, output.String())
			}
		})
	}
}

func TestWorkflowCheckPropagatesEncoderFailure(t *testing.T) {
	t.Parallel()
	cause := errors.New("workflow sink failed")
	err := NewWorkflowCheckCommand().Execute(t.Context(), []string{
		"--workflow", filepath.Join("..", "..", ".github", "workflows", "js-ts-npm-package-slsa3.yml"),
		"--profile", "npm-only",
	}, &stagedWriter{err: cause})
	if !errors.Is(err, cause) {
		t.Fatalf("Execute() = %v, want encoder cause", err)
	}
}

func TestDispatcherReportsUnknownCommand(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	result := NewDispatcher().Dispatch(t.Context(), []string{"unknown"}, &output)
	assertPrimaryDiagnostic(t, output.Bytes(), result.ExitCode, 2, diagnostic.IDVerifierExecutionFailure)
}

func TestHandoffReportsArtifactVerificationFailure(t *testing.T) {
	t.Parallel()
	for _, missing := range []bool{false, true} {
		name := "digest mismatch"
		if missing {
			name = "missing artifact"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			artifact := oneFileDirectory(t, "artifact.bin", []byte("different bytes"))
			if missing {
				artifact = filepath.Join(t.TempDir(), "missing")
			}
			contract := handoff.Handoff{Transport: handoff.TransportGitHubActionsArtifact, ArtifactName: "artifact-1", PayloadFileName: "artifact.bin", PayloadKind: "primary-artifact", Digest: handoff.Digest{Algorithm: handoff.AlgorithmSHA256, Value: digest.SumSHA256([]byte("expected bytes"))}}
			contractPath := filepath.Join(t.TempDir(), "handoff.json")
			if err := os.WriteFile(contractPath, mustMarshalSignJSON(t, contract), 0o600); err != nil {
				t.Fatal(err)
			}
			outputPath := filepath.Join(t.TempDir(), "verified.bin")
			var output bytes.Buffer
			err := NewVerifyHandoffCommand().Execute(t.Context(), []string{"--handoff", contractPath, "--artifact-dir", artifact, "--output", outputPath, "--github-output", ""}, &output)
			want, code, id := ErrVerificationFailure, 1, string(handoff.DigestMismatchID)
			if missing {
				want, code, id = ErrInvocationFailure, 2, diagnostic.IDInputUnavailable
			}
			if !errors.Is(err, want) {
				t.Fatalf("Execute() = %v, want %v", err, want)
			}
			assertPrimaryDiagnostic(t, output.Bytes(), code, code, id)
			if _, err := os.Stat(outputPath); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("verification failure committed payload: %v", err)
			}
		})
	}
}
