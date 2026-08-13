package npmprofile

import (
	"testing"

	"github.com/windlasstech/slsa-builder/internal/attestation"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

func decodeStatementForVerifierTest(t *testing.T, statementJSON []byte) provenance.Statement {
	t.Helper()
	statement, err := provenance.DecodeStatement(statementJSON)
	if err != nil {
		t.Fatal(err)
	}
	return statement
}

func publishVerifierStatement(t *testing.T, mutate func(*ExternalParameters)) (statementInput NPMProvenanceInput, statementBytes []byte) {
	t.Helper()
	input := validProvenanceInput(t, ManagerNPM)
	parameters, err := DecodeExternalParameters(input.BuildMetadata.ExternalParameters)
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(&parameters)
		encoded, err := EncodeExternalParameters(parameters)
		if err != nil {
			t.Fatal(err)
		}
		input.BuildMetadata.ExternalParameters = encoded
	}
	signing, err := NewProvenanceSigningInput(input)
	if err != nil {
		t.Fatal(err)
	}
	return input, signing.StatementJSON
}

func publishVerifierIdentity(ref, revision string) attestation.IdentityExpectation {
	return attestation.IdentityExpectation{
		SignerURI:           "https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@" + testSourceSHA,
		WorkflowSHA:         testSourceSHA,
		SourceRepositoryURI: "https://github.com/example/project",
		SourceRef:           ref,
		SourceDigest:        revision,
	}
}

func TestPublishVerifierBindsCertificateToSignedInvocationRecord(t *testing.T) {
	t.Parallel()

	t.Run("single identity", func(t *testing.T) {
		_, statementJSON := publishVerifierStatement(t, nil)
		statement := decodeStatementForVerifierTest(t, statementJSON)
		verifier := &sigstorePublishVerifier{config: SigstorePublishVerifierConfig{
			Identity: publishVerifierIdentity("refs/tags/v1.2.3", testSourceSHA),
		}}
		if err := verifier.validateStatement(statement); err != nil {
			t.Fatalf("validateStatement() error = %v", err)
		}
	})

	dispatchRetry := func(parameters *ExternalParameters) {
		parameters.Source.InputRef = testStringPointer("refs/tags/v1.2.3")
		parameters.Source.InvocationRef = testStringPointer("refs/heads/main")
		parameters.Source.InvocationRevision = testStringPointer(testAttestSHA)
	}

	t.Run("dispatch retry binds invocation context", func(t *testing.T) {
		_, statementJSON := publishVerifierStatement(t, dispatchRetry)
		statement := decodeStatementForVerifierTest(t, statementJSON)
		verifier := &sigstorePublishVerifier{config: SigstorePublishVerifierConfig{
			Identity: publishVerifierIdentity("refs/heads/main", testAttestSHA),
		}}
		if err := verifier.validateStatement(statement); err != nil {
			t.Fatalf("validateStatement() error = %v", err)
		}
	})

	t.Run("dispatch retry rejects built-identity certificate expectation", func(t *testing.T) {
		_, statementJSON := publishVerifierStatement(t, dispatchRetry)
		statement := decodeStatementForVerifierTest(t, statementJSON)
		verifier := &sigstorePublishVerifier{config: SigstorePublishVerifierConfig{
			Identity: publishVerifierIdentity("refs/tags/v1.2.3", testSourceSHA),
		}}
		requireNPMDiagnostic(t, verifier.validateStatement(statement), "windlass.verify.error.source-digest-mismatch")
	})

	t.Run("invocation ref mismatch", func(t *testing.T) {
		_, statementJSON := publishVerifierStatement(t, dispatchRetry)
		statement := decodeStatementForVerifierTest(t, statementJSON)
		verifier := &sigstorePublishVerifier{config: SigstorePublishVerifierConfig{
			Identity: publishVerifierIdentity("refs/heads/release", testAttestSHA),
		}}
		requireNPMDiagnostic(t, verifier.validateStatement(statement), "windlass.verify.error.source-ref-mismatch")
	})

	t.Run("single identity ref mismatch", func(t *testing.T) {
		_, statementJSON := publishVerifierStatement(t, nil)
		statement := decodeStatementForVerifierTest(t, statementJSON)
		verifier := &sigstorePublishVerifier{config: SigstorePublishVerifierConfig{
			Identity: publishVerifierIdentity("refs/tags/v1.2.4", testSourceSHA),
		}}
		requireNPMDiagnostic(t, verifier.validateStatement(statement), "windlass.verify.error.source-ref-mismatch")
	})
}
