package npmprofile

import (
	"testing"

	"github.com/windlasstech/slsa-builder/internal/provenance"
)

func TestConsumerFinalSourceBindings(t *testing.T) {
	t.Parallel()
	input := sourceRefDispatchRetryProvenanceInput(t)
	statement := runtimeTestStatement(t, input)
	statement.Predicate.BuildDefinition.InternalParameters = mustJSON(provenance.InternalParameters{GitHub: input.GitHub})
	verifier := sigstorePublishVerifier{config: SigstorePublishVerifierConfig{Identity: publishVerifierIdentity("refs/heads/main", testAttestSHA)}}
	verifier.config.Identity.BuildTrigger = "workflow_dispatch"
	if _, err := verifier.identityForStatement(statement); err != nil {
		t.Fatalf("valid dispatch invocation rejected: %v", err)
	}
	verifier.config.Identity.SourceDigest = testSourceSHA
	_, err := verifier.identityForStatement(statement)
	requireNPMDiagnostic(t, err, "windlass.verify.error.source-digest-mismatch")
}

func TestConsumerRequiresAuthenticatedManifest(t *testing.T) {
	t.Parallel()
	input := validProvenanceInput(t, ManagerNPM)
	statement := runtimeTestStatement(t, input)
	for _, test := range []struct {
		name     string
		evidence ManifestEvidence
		valid    bool
	}{
		{"original snapshot", input.BuildMetadata.ManifestEvidence, true},
		{"missing", ManifestEvidence{}, false},
		{"mismatched bytes", ManifestEvidence{Path: "package.json", ContentBase64: "e30="}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			metadata := input.BuildMetadata
			metadata.ManifestEvidence = test.evidence
			verifier := sigstorePublishVerifier{config: SigstorePublishVerifierConfig{Identity: publishVerifierIdentity("refs/tags/v1.2.3", testSourceSHA), BuildMetadata: metadata, Packed: input.Packed}}
			err := verifier.validateStatement(statement)
			if test.valid {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				requireNPMDiagnostic(t, err, IDPackageManifestEvidence)
			}
		})
	}
}

func TestConsumerNormalizedPublishResolution(t *testing.T) {
	t.Parallel()
	intent, err := ResolvePublishIntent(" HTTPS://REGISTRY.NPMJS.ORG:443 ", " next ", " public ", &PublishConfigParameters{Registry: "https://registry.npmjs.org/", Tag: "next", Access: "public"})
	if err != nil {
		t.Fatalf("equivalent normalized publish inputs conflict: %v", err)
	}
	if intent.ResolvedRegistryURL != "https://registry.npmjs.org/" || intent.ResolvedDistTag != "next" || intent.PublishAccessOption == nil || *intent.PublishAccessOption != "public" {
		t.Fatalf("resolution = %#v", intent)
	}
}

func TestPublishPreflightDiagnosticObservations(t *testing.T) {
	fixture := newPublishFixture(t, publishFixtureOptions{initialVersionAbsent: true})
	result, err := Publish(t.Context(), fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	metadata := result.Report.DiagnosticMetadata
	if metadata == nil || metadata.Publish == nil || metadata.Publish.PackageIdentityPreexisting == nil || !*metadata.Publish.PackageIdentityPreexisting || metadata.Publish.PackageVersionPreexisting == nil || *metadata.Publish.PackageVersionPreexisting {
		t.Fatalf("pre-mutation observations missing or replaced by readback: %#v", metadata)
	}
}

func TestConsumerRejectsChangedRawPublishInputs(t *testing.T) {
	t.Parallel()
	input := validProvenanceInput(t, ManagerNPM)
	statement := runtimeTestStatement(t, input)
	parameters := validExternalParameters(ManagerNPM)
	parameters.Publish.InputDistTag = testStringPointer("next")
	statement.Predicate.BuildDefinition.ExternalParameters = mustJSON(parameters)
	verifier := sigstorePublishVerifier{config: SigstorePublishVerifierConfig{Identity: publishVerifierIdentity("refs/tags/v1.2.3", testSourceSHA), BuildMetadata: input.BuildMetadata, Packed: input.Packed}}
	requireNPMDiagnostic(t, verifier.validateStatement(statement), IDUnexpectedExternalParameters)
}
