package npmprofile

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/attestation"
	"github.com/windlasstech/slsa-builder/internal/provenance"
)

func TestPublishStatementRejectsJointlyCorruptedNodeRuntime(t *testing.T) {
	t.Parallel()
	for _, mutation := range []string{"url", "digest", "url and digest"} {
		t.Run(mutation, func(t *testing.T) {
			t.Parallel()
			// Given a received Statement whose runtime differs from the embedded pin.
			input := validProvenanceInput(t, ManagerPNPM)
			for index := range input.BuildMetadata.ResolvedDependencies {
				d := &input.BuildMetadata.ResolvedDependencies[index]
				if d.Name != "node-runtime" {
					continue
				}
				if mutation != "digest" {
					d.URI += ".substituted"
				}
				if mutation != "url" {
					d.Digest["sha256"] = strings.Repeat("0", 64)
				}
			}
			statement := runtimeTestStatement(t, input)
			verifier := sigstorePublishVerifier{config: SigstorePublishVerifierConfig{Identity: publishVerifierIdentity("refs/tags/v1.2.3", testSourceSHA)}}
			// When entering the real publish semantic consumer, it reconstructs accompanying
			// metadata from this SAME corrupted Statement. No signed bundle is rewritten.
			err := verifier.validateStatement(statement)
			// Then equality with that metadata cannot substitute for independent pin validation.
			requireProvenanceDiagnostic(t, err, [2]string{"windlass.verify.error.resolved-dependencies-node-runtime", "resolvedDependencies.node-runtime"})
		})
	}
}

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

func TestRawStatementNodeRuntimeUnknownMember(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, member string
	}{
		{"mediaType", `,"mediaType":"application/gzip"`},
		{"null", `,"unexpected":null`},
		{"Name", `,"Name":"node-runtime"`},
		{"recognized reordered", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			// Given valid Statement bytes with only the runtime descriptor member shape changed.
			input := validProvenanceInput(t, ManagerNPM)
			raw := rawStatementWithRuntimeMember(t, input, test.member)
			statement, err := provenance.DecodeStatement(raw)
			if err != nil {
				t.Fatalf("DecodeStatement() must defer descriptor policy to the profile: %v", err)
			}
			verifier := sigstorePublishVerifier{config: SigstorePublishVerifierConfig{Identity: publishVerifierIdentity("refs/tags/v1.2.3", testSourceSHA)}}
			for _, consumer := range []struct {
				name     string
				validate func() error
			}{
				{"profile", func() error { return ValidateNPMStatement(statement, input) }},
				{"publish", func() error { return verifier.validateStatement(statement) }},
			} {
				t.Run(consumer.name, func(t *testing.T) {
					t.Parallel()
					// When the real semantic consumer validates the decoded Statement.
					err := consumer.validate()
					// Then only exact recognized members are accepted, irrespective of order.
					if test.member == "" {
						if err != nil {
							t.Fatalf("recognized reordered members rejected: %v", err)
						}
						return
					}
					requireProvenanceDiagnostic(t, err, [2]string{"windlass.verify.error.resolved-dependencies-node-runtime", "resolvedDependencies.node-runtime"})
				})
			}
		})
	}
}

func TestCloneNPMDependenciesPreservesUnknownMember(t *testing.T) {
	t.Parallel()
	// Given a decoded runtime descriptor carrying an unknown member.
	raw := rawStatementWithRuntimeMember(t, validProvenanceInput(t, ManagerNPM), `,"mediaType":"application/gzip"`)
	statement, err := provenance.DecodeStatement(raw)
	if err != nil {
		t.Fatal(err)
	}
	// When dependencies cross the production clone boundary.
	cloned := cloneNPMDependencies(statement.Predicate.BuildDefinition.ResolvedDependencies)
	// Then the runtime descriptor still fails closed after cloning.
	for _, descriptor := range cloned {
		if descriptor.Name == "node-runtime" {
			requireProvenanceDiagnostic(t, validateNodeRuntimeDescriptor(descriptor), [2]string{"windlass.verify.error.resolved-dependencies-node-runtime", "resolvedDependencies.node-runtime"})
			return
		}
	}
	t.Fatal("cloned dependencies lost node-runtime")
}

func rawStatementWithRuntimeMember(t *testing.T, input NPMProvenanceInput, member string) []byte {
	t.Helper()
	signing, err := NewProvenanceSigningInput(input)
	if err != nil {
		t.Fatal(err)
	}
	statement := signing.Statement()
	raw := string(mustJSON(statement))
	for _, descriptor := range statement.Predicate.BuildDefinition.ResolvedDependencies {
		if descriptor.Name != "node-runtime" {
			continue
		}
		original := string(mustJSON(descriptor))
		if strings.Count(raw, original) != 1 {
			t.Fatal("expected exactly one runtime descriptor in raw Statement")
		}
		replacement := fmt.Sprintf(`{"annotations":%s,"digest":%s,"uri":%s,"name":"node-runtime"%s}`,
			mustJSON(descriptor.Annotations), mustJSON(descriptor.Digest), mustJSON(descriptor.URI), member)
		return []byte(strings.Replace(raw, original, replacement, 1))
	}
	t.Fatal("valid Statement missing node-runtime")
	return nil
}
