package command

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/attestation"
	"github.com/windlasstech/slsa-builder/internal/policy"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

func TestVerifyAttestationRejectsMalformedTypedInputs(t *testing.T) {
	t.Parallel()

	t.Run("policy", func(t *testing.T) {
		t.Parallel()
		paths := writeAttestationInputs(t, `{"schema_version":1}`, `{}`)
		var output bytes.Buffer
		result := NewDispatcher(NewVerifyAttestationCommand()).Dispatch(context.Background(), paths.args(), &output)
		assertPrimaryDiagnostic(t, output.Bytes(), result.ExitCode, 1, "windlass.verify.error.policy-schema-invalid")
	})

	t.Run("malformed policy JSON", func(t *testing.T) {
		t.Parallel()
		paths := writeAttestationInputs(t, `{"schema_version":`, `{}`)
		var output bytes.Buffer
		result := NewDispatcher(NewVerifyAttestationCommand()).Dispatch(context.Background(), paths.args(), &output)
		assertPrimaryDiagnostic(t, output.Bytes(), result.ExitCode, 1, "windlass.verify.error.policy-schema-invalid")
	})

	t.Run("identity", func(t *testing.T) {
		t.Parallel()
		policyJSON := `{"schema_version":"1","source":{"repository_uri":"https://github.com/example/project","repository_id":"1","repository_owner_id":"2","digest":"0123456789abcdef0123456789abcdef01234567","ref":"refs/tags/v1.0.0"},"producer":{"workflow_path":".github/workflows/build.yml","workflow_sha":"0123456789abcdef0123456789abcdef01234567","runner_environment":"github-hosted"},"trust_root":{"mode":"tuf","instance":"sigstore-public-good"}}`
		paths := writeAttestationInputs(t, policyJSON, `{"unexpected":true}`)
		var output bytes.Buffer
		result := NewDispatcher(NewVerifyAttestationCommand()).Dispatch(context.Background(), paths.args(), &output)
		assertPrimaryDiagnostic(t, output.Bytes(), result.ExitCode, 1, "windlass.verify.error.policy-schema-invalid")
	})
}

type attestationInputPaths struct {
	bundle    string
	policy    string
	identity  string
	statement string
}

func TestBindIdentityPolicySourceRefRebinding(t *testing.T) {
	t.Parallel()

	const (
		builtSHA      = "0123456789abcdef0123456789abcdef01234567"
		invocationSHA = "89abcdef0123456789abcdef0123456789abcdef"
	)
	baseIdentity := attestation.IdentityExpectation{
		Issuer:                  "https://token.actions.githubusercontent.com",
		SignerURI:               "https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@" + builtSHA,
		WorkflowSHA:             builtSHA,
		SourceRepositoryURI:     "https://github.com/example/project",
		SourceRepositoryID:      "1",
		SourceRepositoryOwnerID: "2",
		SourceDigest:            builtSHA,
		SourceRef:               "refs/tags/v1.2.3",
		RunnerEnvironment:       "github-hosted",
	}
	basePolicy := policy.ExplicitPolicy{
		SchemaVersion: "1",
		Source: policy.SourcePolicy{
			RepositoryURI: "https://github.com/example/project", RepositoryID: "1", RepositoryOwnerID: "2",
			Digest: builtSHA, Ref: "refs/tags/v1.2.3",
		},
		Producer: policy.ProducerPolicy{
			WorkflowPath: ".github/workflows/js-ts-npm-package-slsa3.yml", WorkflowSHA: builtSHA, RunnerEnvironment: "github-hosted",
		},
	}
	statementWithSource := func(sourceJSON string) provenance.Statement {
		return provenance.Statement{Predicate: provenance.Predicate{BuildDefinition: provenance.BuildDefinition{
			ExternalParameters: json.RawMessage(`{"source":` + sourceJSON + `}`),
		}}}
	}
	singleIdentity := statementWithSource(`{"repository":"https://github.com/example/project","ref":"refs/tags/v1.2.3","revision":"` + builtSHA + `"}`)
	dispatchRetry := statementWithSource(`{"repository":"https://github.com/example/project","ref":"refs/tags/v1.2.3","revision":"` + builtSHA + `","input_ref":"refs/tags/v1.2.3","invocation_ref":"refs/heads/main","invocation_revision":"` + invocationSHA + `"}`)

	t.Run("single identity", func(t *testing.T) {
		if err := bindIdentityPolicy(baseIdentity, basePolicy, singleIdentity); err != nil {
			t.Fatalf("bindIdentityPolicy() error = %v", err)
		}
	})

	t.Run("dispatch retry", func(t *testing.T) {
		identity := baseIdentity
		identity.SourceRef = "refs/heads/main"
		identity.SourceDigest = invocationSHA
		if err := bindIdentityPolicy(identity, basePolicy, dispatchRetry); err != nil {
			t.Fatalf("bindIdentityPolicy() error = %v", err)
		}
	})

	t.Run("identity bound to built source is rejected", func(t *testing.T) {
		err := bindIdentityPolicy(baseIdentity, basePolicy, dispatchRetry)
		assertBindingDiagnostic(t, err, "windlass.verify.error.source-digest-mismatch")
	})

	t.Run("policy bound to invocation context is rejected", func(t *testing.T) {
		identity := baseIdentity
		identity.SourceRef = "refs/heads/main"
		identity.SourceDigest = invocationSHA
		explicit := basePolicy
		explicit.Source.Ref = "refs/heads/main"
		explicit.Source.Digest = invocationSHA
		err := bindIdentityPolicy(identity, explicit, dispatchRetry)
		assertBindingDiagnostic(t, err, "windlass.verify.error.source-digest-mismatch")
	})

	t.Run("policy repository differs from signed source", func(t *testing.T) {
		explicit := basePolicy
		explicit.Source.RepositoryURI = "https://github.com/example/other"
		identity := baseIdentity
		identity.SourceRepositoryURI = "https://github.com/example/other"
		err := bindIdentityPolicy(identity, explicit, singleIdentity)
		assertBindingDiagnostic(t, err, "windlass.verify.error.source-identity-mismatch")
	})
}

func assertBindingDiagnostic(t *testing.T, err error, wantID string) {
	t.Helper()
	if err == nil {
		t.Fatalf("bindIdentityPolicy() succeeded, want diagnostic %q", wantID)
	}
	if got := diagnosticIDOf(err); got != wantID {
		t.Fatalf("bindIdentityPolicy() diagnostic = %q, want %q (error = %v)", got, wantID, err)
	}
}

func (paths attestationInputPaths) args() []string {
	return []string{"verify-attestation", "--bundle", paths.bundle, "--policy", paths.policy, "--identity", paths.identity, "--statement", paths.statement}
}

func writeAttestationInputs(t *testing.T, policyJSON, identityJSON string) attestationInputPaths {
	t.Helper()
	directory := t.TempDir()
	paths := attestationInputPaths{
		bundle:    filepath.Join(directory, "bundle.json"),
		policy:    filepath.Join(directory, "policy.json"),
		identity:  filepath.Join(directory, "identity.json"),
		statement: filepath.Join(directory, "statement.json"),
	}
	for path, contents := range map[string]string{
		paths.bundle:    `{}`,
		paths.policy:    policyJSON,
		paths.identity:  identityJSON,
		paths.statement: `{}`,
	} {
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return paths
}
