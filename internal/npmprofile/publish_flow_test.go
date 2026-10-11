package npmprofile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/windlasstech/slsa-builder/internal/diagnostic"
	"github.com/windlasstech/slsa-builder/internal/digest"
)

func TestPublishFlowEntryRejections(t *testing.T) {
	// newPublishFixture changes process environment, so these cases must be serial.
	for _, test := range []struct {
		name, message string
		change        func(*publishFixture)
	}{
		{"missing registry", "publish context, registry, and verifier are required", func(f *publishFixture) { f.request.Registry = nil }},
		{"empty registry", "publish context, registry, and verifier are required", func(f *publishFixture) { f.request.Registry = &RegistryClient{} }},
		{"missing verifier", "publish context, registry, and verifier are required", func(f *publishFixture) { f.request.Verifier = nil }},
		{"noncanonical run", "run identity must use positive canonical decimal values", func(f *publishFixture) { f.request.RunID = "01" }},
		{"missing executable", "npm executable is required", func(f *publishFixture) { f.request.NPMExecutable = "" }},
		{"relative executable", `npm executable must be an absolute path to an npm binary: "npm"`, func(f *publishFixture) { f.request.NPMExecutable = "npm" }},
		{"tarball path", "verify tarball path: artifact path must be absolute, canonical, and have the expected suffix", func(f *publishFixture) { f.request.TarballPath = "relative.tgz" }},
		{"tarball digest", "verified tarball handoff digest mismatch", func(f *publishFixture) { f.request.TarballSHA256 = digest.SumSHA256(nil) }},
		{"bundle path", "verify provenance bundle path: artifact path must be absolute, canonical, and have the expected suffix", func(f *publishFixture) { f.request.BundlePath = "relative.intoto.jsonl" }},
		{"bundle digest", "verified provenance bundle handoff digest mismatch", func(f *publishFixture) { f.request.BundleSHA256 = digest.SumSHA256(nil) }},
		{"verifier failure", "verify P02 provenance bundle: unexpected published bundle", func(f *publishFixture) { f.verifier.localBundle = nil }},
		{"statement type", "verified provenance Statement has an unexpected type, build type, or subject count", func(f *publishFixture) { f.verifier.statement.Type = "wrong" }},
		{"signed artifact name", "artifact paths differ from signed package metadata", func(f *publishFixture) { f.request.BundlePath = f.request.TarballPath + ".other.intoto.jsonl" }},
		{"subject digest", "verified provenance subject differs from the tarball handoff", func(f *publishFixture) { f.verifier.statement.Subject[0].Digest["sha256"] = testSHA256 }},
		{"future attempt", "local P02 bundle run identity differs from the current attempt", func(f *publishFixture) {
			f.verifier.statement.Predicate.RunDetails.Metadata.InvocationID = "https://github.com/example/project/actions/runs/123456789/attempts/3"
		}},
		{"wrong workflow", "successful trusted-publisher preflight for the signed caller workflow is required", func(f *publishFixture) { f.request.OIDCExchange.WorkflowFilename = "other.yml" }},
		{"expired token", "trusted-publisher exchange token lifetime is unusable", func(f *publishFixture) { f.request.OIDCExchange.ExpiresAt = f.clock.now }},
		{"future token", "trusted-publisher exchange token lifetime is unusable", func(f *publishFixture) { f.request.OIDCExchange.CreatedAt = f.clock.now.Add(time.Second) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given one broken entry binding and otherwise valid handoffs.
			fixture := newPublishFixture(t, publishFixtureOptions{})
			test.change(fixture)
			if test.name == "signed artifact name" {
				if err := os.WriteFile(fixture.request.BundlePath, fixture.bundleBytes, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			// When entering the public mutation flow.
			result, err := Publish(t.Context(), fixture.request)
			// Then reject before any registry observation or executable invocation.
			requireNPMDiagnostic(t, err, IDBundleByteFormatMismatch)
			if err.Error() != test.message || result.State != PublishIndeterminate || result.MutationAttempted {
				t.Fatalf("Publish() = %#v, %v; want indeterminate/no mutation and %q", result, err, test.message)
			}
			fixture.requireInvocations(t, nil)
			if fixture.packumentRequests != 0 || fixture.attestationRequests != 0 {
				t.Fatal("invalid local handoff reached registry")
			}
		})
	}
}

func TestPublishFlowReadbackConflict(t *testing.T) {
	fixture := newPublishFixture(t, publishFixtureOptions{initialVersionAbsent: true, foreignRunID: true})
	result, err := Publish(t.Context(), fixture.request)
	requireNPMDiagnostic(t, err, IDRegistryLinkageMismatch)
	if err.Error() != "published registry evidence conflicts with the verified handoff" || result.State != PublishForeignConflict || !result.MutationAttempted {
		t.Fatalf("Publish() = %#v, %v; want foreign conflict after one mutation", result, err)
	}
	fixture.requireInvocations(t, fixture.expectedMutationInvocations())
}

func TestPublishFlowPreInvocationRebinding(t *testing.T) {
	for _, test := range []struct{ name, message string }{
		{"tarball", "tarball handoff changed before npm invocation"},
		{"bundle", "bundle handoff changed before npm invocation"},
		{"expiry", "trusted-publisher exchange expired before npm invocation"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given a handoff that passed preparation and subsequently changed.
			fixture := newPublishFixture(t, publishFixtureOptions{})
			prepared, err := preparePublish(t.Context(), fixture.request)
			if err != nil {
				t.Fatal(err)
			}
			switch test.name {
			case "tarball":
				prepared.request.TarballSHA512 = digest.SumSHA512(nil)
			case "bundle":
				prepared.request.BundleSHA256 = digest.SumSHA256(nil)
			case "expiry":
				fixture.clock.now = fixture.request.OIDCExchange.ExpiresAt
			}
			// When attempting the mutation, then no executable is launched.
			output, err := prepared.runNPM(t.Context())
			if err == nil || err.Error() != test.message || len(output) != 0 {
				t.Fatalf("runNPM() = %q, %v; want %q", output, err, test.message)
			}
			fixture.requireInvocations(t, nil)
		})
	}
}

func TestPublishFlowObservationDecisions(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name         string
		change       func(*registryObservation)
		after, final bool
		want         observationDecision
	}{
		{"missing identity at entry", func(o *registryObservation) { o.packageExists = false }, false, false, observationDecision{state: PublishAbsent, terminal: true, packageAbsent: true}},
		{"missing identity after mutation", func(o *registryObservation) { o.packageExists = false }, true, true, observationDecision{state: PublishIndeterminate, terminal: true}},
		{"missing version after mutation", func(o *registryObservation) { o.version = nil }, true, true, observationDecision{state: PublishIndeterminate, terminal: true}},
		{"malformed integrity", func(o *registryObservation) { o.version.Integrity = "sha512-!" }, false, true, observationDecision{state: PublishIndeterminate, terminal: true}},
		{"unverifiable bundle", func(o *registryObservation) {
			o.attestations.Attestations = []RegistryAttestation{{PredicateType: npmSLSAPredicateType, Bundle: []byte("invalid")}}
		}, false, true, observationDecision{state: PublishIndeterminate, terminal: true}},
		{"unrelated predicate", func(o *registryObservation) {
			o.attestations.Attestations = []RegistryAttestation{{PredicateType: "other"}}
		}, false, true, observationDecision{state: PublishForeignConflict, terminal: true}},
		{"missing attestation at deadline", func(o *registryObservation) { o.attestations.Found = false }, false, true, observationDecision{state: PublishForeignConflict, terminal: true}},
		{"wait for attestation", func(*registryObservation) {}, false, false, observationDecision{state: PublishIndeterminate}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			// Given paired registry evidence for matching bytes but incomplete custody.
			prepared := preparedPublish{parameters: validExternalParameters(ManagerNPM), expectedSRI: sha512SRI(digest.SumSHA512(nil)), request: PublishRequest{Verifier: &publishFakeVerifier{}}}
			observation := registryObservation{packageExists: true, version: &RegistryVersion{Integrity: prepared.expectedSRI, Tarball: "https://registry.npmjs.org/" + prepared.parameters.Package.TarballName}, attestations: RegistryAttestationState{Found: true}}
			test.change(&observation)
			// When classifying, then custody and polling bounds determine the state.
			if got := prepared.decide(t.Context(), observation, test.after, test.final); got != test.want {
				t.Fatalf("decide() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestPublishFlowPollingCancellation(t *testing.T) {
	fixture := newPublishFixture(t, publishFixtureOptions{readbackIndeterminate: true})
	fixture.request.Sleep = func(context.Context, time.Duration) error { return context.Canceled }
	result, err := Publish(t.Context(), fixture.request)
	requireNPMDiagnostic(t, err, IDPrepublishRegistryMetadataRequired)
	if !errors.Is(err, context.Canceled) || result.MutationAttempted || result.State != PublishIndeterminate {
		t.Fatalf("Publish() = %#v, %v; want canceled entry observation", result, err)
	}
	fixture.requireInvocations(t, nil)
}

func TestPublishFlowConfigurationDirectoryFailure(t *testing.T) {
	// TMPDIR is process-global. The existing parent remains valid for t.TempDir.
	directory := t.TempDir()
	t.Setenv("TMPDIR", filepath.Join(directory, "missing"))
	prepared := preparedPublish{}
	err := prepared.validateNPMVersion(t.Context())
	if err == nil || err.Error() != "observe npm version before publish: create isolated npm configuration directory" {
		t.Fatalf("validateNPMVersion() = %v; want isolated-directory failure", err)
	}
}

func TestPublishFlowIntegrityAndEnvironment(t *testing.T) {
	t.Parallel()
	for _, input := range []string{"sha256-AA==", "sha512-", " sha512-AA==", "sha512-!", "sha512-AA=="} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			if got, valid := normalizeSHA512SRI(input); valid || got != "" {
				t.Fatalf("normalizeSHA512SRI(%q) = %q, %t; want empty/invalid", input, got, valid)
			}
		})
	}
	got := publishEnvironment([]string{"malformed", "npm_auth_token=secret", "node_options=inject", "npm_config_registry=evil", "PATH=/safe"}, "/isolation", "/user", "/global")
	want := []string{"PATH=/safe", "HOME=/isolation", "USERPROFILE=/isolation", "XDG_CONFIG_HOME=/isolation", "NPM_CONFIG_USERCONFIG=/user", "NPM_CONFIG_GLOBALCONFIG=/global"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("publishEnvironment() = %q, want %q", got, want)
	}
	if got := npmErrorCode([]byte("unrelated E401\nNPM ERR! CODE E403\n")); got != "e403" {
		t.Fatalf("legacy npm error code = %q, want e403", got)
	}
}

func TestPublishFlowPreparedBindings(t *testing.T) {
	for _, test := range []struct {
		name, message string
		change        func(*VerifiedPublishBundle, *ExternalParameters)
	}{
		{"registry", "registry client differs from signed publish intent", func(v *VerifiedPublishBundle, p *ExternalParameters) {
			_ = v
			p.Publish.ResolvedRegistryURL = "https://example.com/"
		}},
		{"certificate invocation", "verified certificate and SLSA invocationId bindings differ", func(v *VerifiedPublishBundle, p *ExternalParameters) { _ = p; v.RunInvocationURI = "" }},
		{"subject count", "verified provenance Statement has an unexpected type, build type, or subject count", func(v *VerifiedPublishBundle, p *ExternalParameters) { _ = p; v.Statement.Subject = nil }},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newPublishFixture(t, publishFixtureOptions{})
			verified := VerifiedPublishBundle{Statement: fixture.verifier.statement, RunInvocationURI: fixture.verifier.statement.Predicate.RunDetails.Metadata.InvocationID}
			parameters := validExternalParameters(ManagerNPM)
			test.change(&verified, &parameters)
			got, err := validatePreparedBindings(fixture.request, verified, parameters)
			if got != "" || err == nil || err.Error() != test.message {
				t.Fatalf("validatePreparedBindings() = %q, %v; want %q", got, err, test.message)
			}
			if test.name != "registry" && (preparedPublish{request: fixture.request}).matchesPublishedBundle(verified) {
				t.Fatal("malformed published binding was accepted")
			}
		})
	}
}

func TestPublishFlowPreparationIntent(t *testing.T) {
	fixture := newPublishFixture(t, publishFixtureOptions{})
	parameters := validExternalParameters(ManagerNPM)
	parameters.Publish.PublishAccessOption = testStringPointer("public")
	parameters.Publish.EffectiveAccess = "public"
	encoded, err := EncodeExternalParameters(parameters)
	if err != nil {
		t.Fatal(err)
	}
	fixture.verifier.statement.Predicate.BuildDefinition.ExternalParameters = encoded
	fixture.request.Sleep = nil
	prepared, err := preparePublish(t.Context(), fixture.request)
	if err != nil || !reflect.DeepEqual(prepared.argv, append(fixture.expectedArgv(), "--access=public")) || prepared.sleep == nil {
		t.Fatalf("preparePublish() = %#v, %v; want explicit access argv and default sleeper", prepared.argv, err)
	}
}

func TestPublishFlowMalformedParameters(t *testing.T) {
	fixture := newPublishFixture(t, publishFixtureOptions{})
	fixture.verifier.statement.Predicate.BuildDefinition.ExternalParameters = []byte(`{}`)
	result, err := Publish(t.Context(), fixture.request)
	requireNPMDiagnostic(t, err, IDUnexpectedExternalParameters)
	if result.MutationAttempted {
		t.Fatal("malformed parameters permitted mutation")
	}
	fixture.requireInvocations(t, nil)
}

func TestPublishFlowReportContractFailures(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, message string
		invoke        func() (PublishResult, error)
	}{
		{"missing success invocation", "run_invocation must be non-empty when present", func() (PublishResult, error) { return (preparedPublish{}).success(false) }},
		{"unknown failure ID", `unknown diagnostic ID "unknown"`, func() (PublishResult, error) {
			return publishFailureWithInvocation("", PublishAbsent, false, "unknown", "test", context.Canceled)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := test.invoke()
			var contract *diagnostic.ContractError
			if !errors.As(err, &contract) || contract.ID != "windlass.verify.error.diagnostics-contract-invalid" || contract.Reason != test.message || !reflect.DeepEqual(got, PublishResult{}) {
				t.Fatalf("report construction = %#v, %v; want contract failure %q", got, err, test.message)
			}
		})
	}
}
