package command

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
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
