package command

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/attestation"
	"github.com/windlasstech/slsa-builder/internal/diagnostic"
	"github.com/windlasstech/slsa-builder/internal/policy"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

func TestVerifyAttestationClassifiesUnavailableInputs(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"bundle", "policy", "identity", "statement"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			paths := writeSourceBindingInputs(t, sourceBindingPolicy("https://github.com/example/project", signBuiltRef, commandBuiltSHA), sourceBindingIdentity("https://github.com/example/project", "1", "2", signInvocationRef, commandInvocationSHA), sourceBindingStatement(t, sourceBindingStatementOptions{}))
			path := map[string]string{"bundle": paths.bundle, "policy": paths.policy, "identity": paths.identity, "statement": paths.statement}[stage]
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			err := NewVerifyAttestationCommand().Execute(t.Context(), paths.args()[1:], &output)
			if !errors.Is(err, ErrInvocationFailure) {
				t.Fatalf("Execute() = %v, want invocation sentinel", err)
			}
			assertPrimaryDiagnostic(t, output.Bytes(), 2, 2, diagnostic.IDInputUnavailable)
			report, err := decodeTypedJSON[Report](bytes.TrimSpace(output.Bytes()), nil)
			if err != nil || len(report.Diagnostics) != 1 || report.Diagnostics[0].Check != stage {
				t.Fatalf("report = %#v, error = %v, want check %q", report, err, stage)
			}
		})
	}
}

func TestVerifyAttestationRejectsAmbiguousIdentityAndMalformedStatement(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, content, id string }{
		{"identity", `{"issuer":"first","issuer":"second"}`, "windlass.verify.error.duplicate-json-member"},
		{"statement", `{"predicate":42}`, "windlass.verify.error.statement-assembly-mismatch"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			explicit := sourceBindingPolicy("https://github.com/example/project", signBuiltRef, commandBuiltSHA)
			paths := writeAttestationInputs(t, string(mustMarshalSignJSON(t, explicit)), `{}`)
			path := paths.statement
			if test.name == "identity" {
				path = paths.identity
			}
			if err := os.WriteFile(path, []byte(test.content), 0o600); err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			err := NewVerifyAttestationCommand().Execute(t.Context(), paths.args()[1:], &output)
			if !errors.Is(err, ErrVerificationFailure) {
				t.Fatalf("Execute() = %v, want verification sentinel", err)
			}
			assertPrimaryDiagnostic(t, output.Bytes(), 1, 1, test.id)
		})
	}
}

func TestVerifyAttestationEnforcesTrustRootMode(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"missing flag", "missing file", "offline malformed bundle", "online pinned root"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			explicit := sourceBindingPolicy("https://github.com/example/project", signBuiltRef, commandBuiltSHA)
			rootPath := filepath.Join(t.TempDir(), "root.json")
			if stage != "online pinned root" {
				explicit.TrustRoot = policy.TrustRoot{Mode: "pinned", Instance: "sigstore-public-good", Path: new("root.json"), SHA256: new(strings.Repeat("0", 64)), TUFRepository: new("https://tuf-repo-cdn.sigstore.dev"), RevalidatedAt: new("2026-08-05T00:00:00Z"), RefreshBefore: new("2026-08-12T00:00:00Z")}
			}
			paths := writeSourceBindingInputs(t, explicit, sourceBindingIdentity("https://github.com/example/project", "1", "2", signInvocationRef, commandInvocationSHA), sourceBindingStatement(t, sourceBindingStatementOptions{}))
			args := paths.args()[1:]
			if stage != "missing flag" {
				args = append(args, "--pinned-root", rootPath)
			}
			if stage == "offline malformed bundle" {
				if err := os.WriteFile(rootPath, []byte(`{}`), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var output bytes.Buffer
			err := NewVerifyAttestationCommand().Execute(t.Context(), args, &output)
			want, code, id := ErrInvocationFailure, 2, diagnostic.IDInputUnavailable
			switch stage {
			case "online pinned root":
				id = "windlass.verify.error.verification-mode-invalid"
			case "offline malformed bundle":
				want, code, id = ErrVerificationFailure, 1, "windlass.verify.error.signature-invalid"
			}
			if !errors.Is(err, want) {
				t.Fatalf("Execute() = %v, want %v", err, want)
			}
			assertPrimaryDiagnostic(t, output.Bytes(), code, code, id)
		})
	}
}

func TestIdentityBindingRejectsRemainingConflicts(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"parameters", "workflow SHA", "signer path"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			statement, err := provenance.DecodeStatement(sourceBindingStatement(t, sourceBindingStatementOptions{}))
			if err != nil {
				t.Fatal(err)
			}
			identity := sourceBindingIdentity("https://github.com/example/project", "1", "2", signInvocationRef, commandInvocationSHA)
			want := ""
			switch stage {
			case "parameters":
				statement.Predicate.BuildDefinition.ExternalParameters, want = []byte(`{}`), "unexpected-external-parameters"
			case "workflow SHA":
				identity.WorkflowSHA, want = signOtherBuiltRevision, "identity expectation conflicts"
			case "signer path":
				identity.SignerURI, want = "https://github.com/example/project/.github/workflows/other.yml@refs/heads/main", "signer URI conflicts"
			}
			got, err := bindIdentityPolicy(identity, sourceBindingPolicy("https://github.com/example/project", signBuiltRef, commandBuiltSHA), statement)
			if err == nil || !strings.Contains(err.Error(), want) || got != (attestation.IdentityExpectation{}) {
				t.Fatalf("binding = %#v, error = %v, want zero identity and %q", got, err, want)
			}
		})
	}
}

func TestVerifyAttestationReportsUnclassifiedBindingConflict(t *testing.T) {
	t.Parallel()
	identity := sourceBindingIdentity("https://github.com/example/project", "1", "2", signInvocationRef, commandInvocationSHA)
	identity.WorkflowSHA = signOtherBuiltRevision
	paths := writeSourceBindingInputs(t, sourceBindingPolicy("https://github.com/example/project", signBuiltRef, commandBuiltSHA), identity, sourceBindingStatement(t, sourceBindingStatementOptions{}))
	var output bytes.Buffer
	err := NewVerifyAttestationCommand().Execute(t.Context(), paths.args()[1:], &output)
	if !errors.Is(err, ErrVerificationFailure) {
		t.Fatalf("Execute() = %v, want verification sentinel", err)
	}
	assertPrimaryDiagnostic(t, output.Bytes(), 1, 1, "windlass.verify.error.policy-schema-invalid")
}
