package npmprofile

import (
	"bytes"
	"testing"
)

func TestPublishPinAllowsDifferentBuildVersion(t *testing.T) {
	// Given independent build and publish npm versions.
	fixture := newPublishFixture(t, publishFixtureOptions{initialVersionAbsent: true, npmVersion: "11.21.0", buildNPMVersion: "11.19.0"})
	before := bytes.Clone(fixture.verifier.statement.Predicate.BuildDefinition.ExternalParameters)
	// When
	result, err := Publish(t.Context(), fixture.request)
	// Then
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if result.State != PublishCommittedAsExpected || !result.MutationAttempted {
		t.Fatalf("Publish() = %#v, want committed mutation", result)
	}
	fixture.requireInvocations(t, fixture.expectedMutationInvocations())
	if !bytes.Equal(before, fixture.verifier.statement.Predicate.BuildDefinition.ExternalParameters) {
		t.Fatal("publish changed build-time external parameters")
	}
}

func TestPublishPinRejectsMatchingUnpinnedBuildVersion(t *testing.T) {
	// Given matching versions that are not the publish pin.
	fixture := newPublishFixture(t, publishFixtureOptions{initialVersionAbsent: true, npmVersion: "11.20.0", buildNPMVersion: "11.20.0"})
	// When
	result, err := Publish(t.Context(), fixture.request)
	// Then
	if err == nil {
		t.Fatal("Publish() error = nil, want publish-npm-pin-mismatch")
	}
	requireNPMDiagnostic(t, err, "windlass.verify.error.publish-npm-pin-mismatch")
	if result.MutationAttempted {
		t.Fatal("publish attempted mutation with an unpinned npm")
	}
	fixture.requireInvocations(t, [][]string{{"--version"}})
	if got := result.Report.Diagnostics[0].Evidence["publish_npm_version"]; got != "11.20.0" {
		t.Fatalf("publish npm evidence = %v, want 11.20.0", got)
	}
	if result.Report.Diagnostics[0].Check != "npm.publish.toolchain" {
		t.Fatalf("check = %q", result.Report.Diagnostics[0].Check)
	}
	if result.Report.DiagnosticMetadata == nil || result.Report.DiagnosticMetadata.PublishNPMVersion == nil || *result.Report.DiagnosticMetadata.PublishNPMVersion != "11.20.0" {
		t.Fatalf("pin mismatch lost observed metadata: %#v", result.Report.DiagnosticMetadata)
	}
}

func TestPublishPinPreservesMetadataOnLaterFailure(t *testing.T) {
	for _, tc := range []struct {
		name    string
		options publishFixtureOptions
		wantID  string
	}{
		{"mutation", publishFixtureOptions{initialVersionAbsent: true, npmExitCode: 1, npmError: "npm error code E403"}, IDMutationPermissionDenied},
		{"readback", publishFixtureOptions{initialVersionAbsent: true, readbackIndeterminate: true}, IDRegistryLinkageMismatch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Given
			fixture := newPublishFixture(t, tc.options)
			// When
			result, err := Publish(t.Context(), fixture.request)
			// Then
			requireNPMDiagnostic(t, err, tc.wantID)
			if result.Report.DiagnosticMetadata == nil || result.Report.DiagnosticMetadata.PublishNPMVersion == nil || *result.Report.DiagnosticMetadata.PublishNPMVersion != "11.21.0" {
				t.Fatalf("failure lost observed metadata: %#v", result.Report.DiagnosticMetadata)
			}
			fixture.requireInvocations(t, fixture.expectedMutationInvocations())
		})
	}
}

func TestPublishPinObservationFailurePreservesDiagnostic(t *testing.T) {
	for _, tc := range []struct {
		name    string
		version string
		exit    string
	}{
		{"command-failed", "11.21.0", "1"},
		{"unparseable", "not-a-version", "0"},
		{"empty", "", "0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Given
			fixture := newPublishFixture(t, publishFixtureOptions{initialVersionAbsent: true})
			t.Setenv("FAKE_NPM_VERSION", tc.version)
			t.Setenv("FAKE_NPM_VERSION_EXIT", tc.exit)
			// When
			result, err := Publish(t.Context(), fixture.request)
			// Then
			requireNPMDiagnostic(t, err, "windlass.verify.error.builder-version-mismatch")
			if result.MutationAttempted || result.Report.DiagnosticMetadata != nil {
				t.Fatalf("failed observation fabricated metadata or attempted mutation: %#v", result)
			}
			fixture.requireInvocations(t, [][]string{{"--version"}})
		})
	}
}

func TestPublishPinOmitsUnobservedMetadataOnRetry(t *testing.T) {
	// Given
	fixture := newPublishFixture(t, publishFixtureOptions{})
	// When
	result, err := Publish(t.Context(), fixture.request)
	// Then
	if err != nil || result.Report.DiagnosticMetadata != nil {
		t.Fatalf("read-only retry = %#v, %v", result, err)
	}
	fixture.requireInvocations(t, nil)
}

func TestPublishPinRecordsSuccessMetadata(t *testing.T) {
	// Given a publish pin equal to the build record, isolating the evidence change.
	fixture := newPublishFixture(t, publishFixtureOptions{initialVersionAbsent: true, npmVersion: "11.21.0", buildNPMVersion: "11.21.0"})
	// When
	result, err := Publish(t.Context(), fixture.request)
	// Then
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := result.Report.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(encoded, []byte(`"diagnostic_metadata":{"publish_npm_version":"11.21.0"}`)) {
		t.Fatalf("missing observed publish npm metadata: %s", encoded)
	}
}
