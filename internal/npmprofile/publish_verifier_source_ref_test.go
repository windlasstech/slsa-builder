package npmprofile

import (
	"testing"

	"github.com/windlasstech/slsa-builder/internal/attestation"
)

func TestPublishVerifierBindsCertificateToSignedInvocationRecord(t *testing.T) {
	singleIdentity := validProvenanceInput(t, ManagerPNPM)
	dispatchRetry := sourceRefDispatchRetryProvenanceInput(t)

	tests := []struct {
		name     string
		input    NPMProvenanceInput
		identity attestation.IdentityExpectation
		wantID   string
	}{
		{
			name:     "single identity accepted",
			input:    singleIdentity,
			identity: publishVerifierIdentity("refs/tags/v1.2.3", testSourceSHA),
		},
		{
			name:     "dispatch retry certificate invocation accepted",
			input:    dispatchRetry,
			identity: publishVerifierIdentity("refs/heads/main", testAttestSHA),
		},
		{
			name:     "dispatch retry certificate built context rejected",
			input:    dispatchRetry,
			identity: publishVerifierIdentity("refs/tags/v1.2.3", testSourceSHA),
			wantID:   "windlass.verify.error.source-ref-mismatch",
		},
		{
			name:     "invocation ref mismatch",
			input:    dispatchRetry,
			identity: publishVerifierIdentity("refs/heads/release", testAttestSHA),
			wantID:   "windlass.verify.error.source-ref-mismatch",
		},
		{
			name:     "invocation revision mismatch",
			input:    dispatchRetry,
			identity: publishVerifierIdentity("refs/heads/main", testSourceSHA),
			wantID:   "windlass.verify.error.source-digest-mismatch",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			signing, err := NewProvenanceSigningInput(test.input)
			if err != nil {
				t.Fatalf("NewProvenanceSigningInput() error = %v", err)
			}
			verifier := sigstorePublishVerifier{config: SigstorePublishVerifierConfig{Identity: test.identity}}
			_, err = verifier.identityForStatement(signing.Statement())
			if test.wantID == "" {
				if err != nil {
					t.Fatalf("identityForStatement() error = %v", err)
				}
				if err := verifier.validateStatement(signing.Statement()); err != nil {
					t.Fatalf("validateStatement() error = %v", err)
				}
				return
			}
			requireNPMDiagnostic(t, err, test.wantID)
		})
	}
}

func publishVerifierIdentity(sourceRef, sourceDigest string) attestation.IdentityExpectation {
	return attestation.IdentityExpectation{
		SignerURI:           "https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@refs/heads/main",
		WorkflowSHA:         testSourceSHA,
		SourceRepositoryURI: "https://github.com/example/project",
		SourceDigest:        sourceDigest,
		SourceRef:           sourceRef,
	}
}
