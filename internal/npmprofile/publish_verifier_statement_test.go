package npmprofile

import (
	"encoding/json"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/attestation"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

func TestPublishStatementValidationGuards(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, message, id string
		change            func(*provenance.Statement, *attestation.IdentityExpectation)
	}{
		{"malformed parameters", "", IDUnexpectedExternalParameters, func(s *provenance.Statement, i *attestation.IdentityExpectation) {
			_ = i
			s.Predicate.BuildDefinition.ExternalParameters = json.RawMessage(`{}`)
		}},
		{"repository differs", "signed npm source repository differs from verified Fulcio identity", "", func(s *provenance.Statement, i *attestation.IdentityExpectation) {
			_ = s
			i.SourceRepositoryURI = "https://github.com/other/project"
		}},
		{"builder separator absent", "signed npm builder identity differs from verified Fulcio signer", "", func(s *provenance.Statement, i *attestation.IdentityExpectation) {
			_ = i
			s.Predicate.RunDetails.Builder.ID = NPMWorkflowPath
		}},
		{"signer separator absent", "signed npm builder identity differs from verified Fulcio signer", "", func(s *provenance.Statement, i *attestation.IdentityExpectation) {
			_ = s
			i.SignerURI = NPMWorkflowPath
		}},
		{"signer workflow differs", "signed npm builder identity differs from verified Fulcio signer", "", func(s *provenance.Statement, i *attestation.IdentityExpectation) {
			_ = s
			i.SignerURI = "https://github.com/other/project/.github/workflows/release.yml@refs/heads/main"
		}},
		{"builder revision differs", "signed npm builder identity differs from verified Fulcio signer", "", func(s *provenance.Statement, i *attestation.IdentityExpectation) {
			_ = s
			i.WorkflowSHA = testAttestSHA
		}},
		{"no subject", "signed npm provenance must contain exactly one subject", "", func(s *provenance.Statement, i *attestation.IdentityExpectation) { _ = i; s.Subject = nil }},
		{"multiple subjects", "signed npm provenance must contain exactly one subject", "", func(s *provenance.Statement, i *attestation.IdentityExpectation) {
			_ = i
			s.Subject = append(s.Subject, s.Subject[0])
		}},
		{"corepack absent", "signed npm builder is missing required Corepack version", "", func(s *provenance.Statement, i *attestation.IdentityExpectation) {
			_ = i
			delete(s.Predicate.RunDetails.Builder.Version, "corepack")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			// Given an assembled valid pnpm Statement with one violated binding.
			signing, err := NewProvenanceSigningInput(validProvenanceInput(t, ManagerPNPM))
			if err != nil {
				t.Fatalf("assemble valid Statement: %v", err)
			}
			statement := signing.Statement()
			identity := publishVerifierIdentity("refs/tags/v1.2.3", testSourceSHA)
			test.change(&statement, &identity)
			verifier := sigstorePublishVerifier{config: SigstorePublishVerifierConfig{Identity: identity}}
			// When checking signed semantics, then reject the precise violated binding.
			err = verifier.validateStatement(statement)
			if test.id != "" {
				requireNPMDiagnostic(t, err, test.id)
			} else if err == nil || err.Error() != test.message {
				t.Fatalf("validateStatement() = %v, want %q", err, test.message)
			}
		})
	}
}

func TestPublishStatementNPMDoesNotRequireCorepack(t *testing.T) {
	t.Parallel()
	signing, err := NewProvenanceSigningInput(validProvenanceInput(t, ManagerNPM))
	if err != nil {
		t.Fatalf("assemble npm Statement: %v", err)
	}
	statement := signing.Statement()
	if _, present := statement.Predicate.RunDetails.Builder.Version["corepack"]; present {
		t.Fatal("npm fixture unexpectedly includes Corepack")
	}
	verifier := sigstorePublishVerifier{config: SigstorePublishVerifierConfig{Identity: publishVerifierIdentity("refs/tags/v1.2.3", testSourceSHA)}}
	if err := verifier.validateStatement(statement); err != nil {
		t.Fatalf("valid npm Statement without Corepack rejected: %v", err)
	}
}
