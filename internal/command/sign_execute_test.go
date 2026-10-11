package command

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/windlasstech/slsa-builder/internal/diagnostic"
	"github.com/windlasstech/slsa-builder/internal/digest"
	"github.com/windlasstech/slsa-builder/internal/npmprofile"
	"github.com/windlasstech/slsa-builder/internal/signing"
)

func TestSignRejectsInputsBeforePreflight(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, flag, value, want string }{
		{"metadata digest", "--metadata-sha256", "invalid", "metadata SHA-256 is invalid"},
		{"tarball digest", "--tarball-sha256", "invalid", "tarball SHA-256 is invalid"},
		{"metadata binding", "--metadata-sha256", strings.Repeat("0", 64), "verify build metadata handoff"},
		{"tarball binding", "--tarball-sha256", strings.Repeat("0", 64), "verify tarball handoff"},
		{"package mismatch", "--package-name", "other", "signing preflight inputs differ"},
		{"registry mismatch", "--registry-url", "https://registry.example/", "signing preflight inputs differ"},
		{"missing built revision", "--built-revision", "", "must be supplied together"},
		{"missing built ref", "--built-ref", "", "must be supplied together"},
		{"malformed metadata", "", "", "decode build metadata"},
		{"artifact name mismatch", "", "", "verified tarball differs"},
		{"sha256 mismatch", "", "", "verified tarball differs"},
		{"sha512 mismatch", "", "", "verified tarball differs"},
		{"malformed parameters", "", "", "unexpected-external-parameters"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			metadata, tarball := signSourceRefMetadata(t)
			switch test.name {
			case "artifact name mismatch":
				metadata.PrimaryArtifact.ArtifactName = "other"
			case "sha256 mismatch":
				metadata.PrimaryArtifact.SHA256 = strings.Repeat("0", 64)
			case "sha512 mismatch":
				metadata.PrimaryArtifact.SHA512 = strings.Repeat("0", 128)
			case "malformed parameters":
				metadata.ExternalParameters = []byte(`{}`)
			}
			encoded := mustMarshalSignJSON(t, metadata)
			if test.name == "malformed metadata" {
				encoded = []byte(`{"unexpected":true}`)
			}
			args := signSourceRefArguments(writeSignHandoff(t, "build-metadata.json", encoded), writeSignHandoff(t, signTarballName, tarball), t.TempDir(), filepath.Join(t.TempDir(), "output"), digest.SumSHA256(encoded).String(), digest.SumSHA256(tarball).String(), signBuiltRef, signBuiltRevision)
			if test.flag != "" {
				args = append(args, test.flag, test.value)
			}
			command := npmProfileSignCommand{preflight: func(context.Context, string, string) npmprofile.OIDCExchangeResult {
				t.Fatal("preflight called for invalid signing input")
				return npmprofile.OIDCExchangeResult{}
			}}
			var output bytes.Buffer
			err := command.Execute(t.Context(), args, &output)
			if err == nil || !strings.Contains(err.Error(), test.want) || output.Len() != 0 {
				t.Fatalf("Execute() = %v, output = %q, want %q before preflight", err, output.String(), test.want)
			}
		})
	}
}

func TestSignExecutionFailureBoundaries(t *testing.T) {
	// These subtests set trusted GitHub environment observations and must be serial.
	for _, stage := range []string{"default preflight", "preflight report", "preflight writer", "caller mismatch", "policy writer", "runtime identity", "missing directory", "nonempty directory", "bundle permission", "github output", "report writer"} {
		t.Run(stage, func(t *testing.T) {
			metadata, tarball := signSourceRefMetadata(t)
			encoded := mustMarshalSignJSON(t, metadata)
			outputDirectory, githubOutput := t.TempDir(), filepath.Join(t.TempDir(), "output")
			setSignGitHubEnvironment(t, signInvocationRef, signInvocationRevision, githubOutput)
			cause := errors.New("sign report sink failed")
			var output bytes.Buffer
			var writer io.Writer = &output
			want := ""
			wantSigned := false
			command := npmProfileSignCommand{
				preflight: func(context.Context, string, string) npmprofile.OIDCExchangeResult {
					return npmprofile.OIDCExchangeResult{WorkflowFilename: "release.yml"}
				},
				now: func() time.Time { return time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC) },
			}
			signed := false
			command.sign = func(ctx context.Context, request signing.Request) (signing.Result, error) {
				if err := ctx.Err(); err != nil {
					return signing.Result{}, err
				}
				signed = true
				return signing.Result{Statement: request.Statement, Bundle: []byte(`{"bundle":"fixture"}`)}, nil
			}
			switch stage {
			case "default preflight":
				t.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", "")
				t.Setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", "")
				command.preflight = nil
			case "preflight report", "preflight writer":
				entry, err := diagnostic.New(npmprofile.IDNPMOIDCExchangeIndeterminate, "preflight", "exchange unavailable")
				if err != nil {
					t.Fatal(err)
				}
				report, err := diagnostic.Build(nil, []diagnostic.Diagnostic{entry}, nil)
				if err != nil {
					t.Fatal(err)
				}
				command.preflight = func(context.Context, string, string) npmprofile.OIDCExchangeResult {
					return npmprofile.OIDCExchangeResult{Report: report}
				}
				if stage == "preflight writer" {
					writer = &stagedWriter{err: cause}
				}
			case "caller mismatch":
				command.preflight = func(context.Context, string, string) npmprofile.OIDCExchangeResult {
					return npmprofile.OIDCExchangeResult{WorkflowFilename: "other.yml"}
				}
				want = "OIDC caller workflow differs"
			case "policy writer":
				writer = &stagedWriter{err: cause}
			case "runtime identity":
				t.Setenv("GITHUB_REPOSITORY_ID", "")
				want = "required trusted runtime value GITHUB_REPOSITORY_ID"
			case "missing directory":
				outputDirectory = filepath.Join(outputDirectory, "missing")
				want, wantSigned = "existing non-symlink directory", true
			case "nonempty directory":
				if err := os.WriteFile(filepath.Join(outputDirectory, "existing"), []byte("preserve"), 0o600); err != nil {
					t.Fatal(err)
				}
				want, wantSigned = "readable and empty", true
			case "bundle permission":
				if os.Geteuid() == 0 {
					t.Skip("root bypasses Unix permission checks")
				}
				if err := os.Chmod(outputDirectory, 0o500); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.Chmod(outputDirectory, 0o700); err != nil {
						t.Error(err)
					}
				})
				want, wantSigned = "write verified provenance bundle", true
			case "github output":
				githubOutput, want, wantSigned = t.TempDir(), "regular non-symlink file", true
			case "report writer":
				writer, wantSigned = &stagedWriter{err: cause}, true
			}
			args := signSourceRefArguments(writeSignHandoff(t, "build-metadata.json", encoded), writeSignHandoff(t, signTarballName, tarball), outputDirectory, githubOutput, digest.SumSHA256(encoded).String(), digest.SumSHA256(tarball).String(), signBuiltRef, signBuiltRevision)
			if stage == "policy writer" {
				args = append(args, "--built-revision", signOtherBuiltRevision)
			}
			err := command.Execute(t.Context(), args, writer)
			if signed != wantSigned {
				t.Fatalf("signer called = %t, want %t", signed, wantSigned)
			}
			switch stage {
			case "default preflight":
				if !errors.Is(err, ErrVerificationFailure) {
					t.Fatalf("Execute() = %v, want verification sentinel", err)
				}
				assertPrimaryDiagnostic(t, output.Bytes(), 1, npmprofile.IDOIDCCapabilityUnavailable)
			case "preflight report":
				if !errors.Is(err, ErrVerificationFailure) {
					t.Fatalf("Execute() = %v, want verification sentinel", err)
				}
				assertPrimaryDiagnostic(t, output.Bytes(), 1, npmprofile.IDNPMOIDCExchangeIndeterminate)
			case "preflight writer", "policy writer", "report writer":
				if !errors.Is(err, cause) {
					t.Fatalf("Execute() = %v, want writer cause", err)
				}
			default:
				if err == nil || !strings.Contains(err.Error(), want) || output.Len() != 0 {
					t.Fatalf("Execute() = %v, output = %q, want %q", err, output.String(), want)
				}
				if wantSigned && stage != "github output" && !strings.Contains(err.Error(), "statement-sha256=") {
					t.Fatalf("failure lost Statement digest: %v", err)
				}
			}
		})
	}
}

func TestSigningIdentityRequiresEveryTrustedObservation(t *testing.T) {
	for _, name := range []string{"GITHUB_SERVER_URL", "GITHUB_REPOSITORY", "GITHUB_REPOSITORY_ID", "GITHUB_REPOSITORY_OWNER_ID", "GITHUB_SHA", "GITHUB_REF", "WINDLASS_WORKFLOW_SHA"} {
		t.Run(name, func(t *testing.T) {
			setSignGitHubEnvironment(t, signBuiltRef, signBuiltRevision, filepath.Join(t.TempDir(), "output"))
			t.Setenv(name, "")
			_, err := githubSigningIdentity(signRunInvocation)
			if err == nil || err.Error() != "required trusted runtime value "+name+" is unavailable" {
				t.Fatalf("githubSigningIdentity() = %v, want missing %s", err, name)
			}
		})
	}
}
