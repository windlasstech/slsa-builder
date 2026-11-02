package npmprofile

import (
	"errors"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/digest"
)

func TestPublishObservationSummary(t *testing.T) {
	expectedSRI := sha512SRI(digest.SumSHA512(nil))
	foreignSRI := sha512SRI(digest.SumSHA512([]byte("foreign tarball")))
	packumentErr := errors.New("packument secret canary")
	attestationErr := errors.New("attestation secret canary")
	matching := registryObservation{
		packageExists: true,
		version:       &RegistryVersion{Integrity: expectedSRI, Tarball: "https://registry.invalid/raw-url-canary"},
		attestations:  RegistryAttestationState{Found: true},
	}

	t.Run("return to first", func(t *testing.T) {
		// Given a changed observation followed by a return to the first observation.
		summary := newPublishObservationSummary(expectedSRI)
		changed := registryObservation{
			packageExists: true,
			version:       &RegistryVersion{Integrity: foreignSRI},
			attestations:  RegistryAttestationState{Found: false},
		}
		summary.record(matching)
		summary.record(changed)
		summary.record(matching)

		// When formatting the final classification boundary.
		line := summary.format(true, PublishCommittedAsExpected)

		// Then the latest difference from the first remains bounded and counted once.
		for _, want := range []string{
			"after_mutation=true observations=3 changes=1",
			`first="package=present version=present integrity=match attestations=present errors=none"`,
			`latest_changed="package=present version=present integrity=mismatch attestations=absent errors=none"`,
			`final="package=present version=present integrity=match attestations=present errors=none"`,
			"decision=committed-as-expected",
		} {
			if !strings.Contains(line, want) {
				t.Errorf("return-to-first summary missing %q\n%s", want, line)
			}
		}
	})

	t.Run("observation classes", func(t *testing.T) {
		mismatched := matching
		mismatched.version = &RegistryVersion{Integrity: foreignSRI}
		mismatched.attestationErr = attestationErr
		malformed := matching
		malformed.version = &RegistryVersion{Integrity: "sha512-!"}
		malformed.attestations = RegistryAttestationState{}
		packumentFailure := matching
		packumentFailure.packumentErr = packumentErr
		bothFailures := malformed
		bothFailures.packumentErr = packumentErr
		bothFailures.attestationErr = attestationErr

		for _, test := range []struct {
			name        string
			observation registryObservation
			want        string
		}{
			{name: "match and no errors", observation: matching, want: "package=present version=present integrity=match attestations=present errors=none"},
			{name: "package absent", observation: registryObservation{}, want: "package=absent version=absent integrity=unknown attestations=absent errors=none"},
			{name: "version absent", observation: registryObservation{packageExists: true}, want: "package=present version=absent integrity=unknown attestations=absent errors=none"},
			{name: "packument error", observation: packumentFailure, want: "package=unknown version=unknown integrity=unknown attestations=present errors=packument"},
			{name: "attestation error and mismatch", observation: mismatched, want: "package=present version=present integrity=mismatch attestations=unknown errors=attestation"},
			{name: "both errors", observation: bothFailures, want: "package=unknown version=unknown integrity=unknown attestations=unknown errors=both"},
			{name: "malformed integrity", observation: malformed, want: "package=present version=present integrity=malformed attestations=absent errors=none"},
		} {
			t.Run(test.name, func(t *testing.T) {
				// Given one observation with a closed safe-state and error class.
				summary := newPublishObservationSummary(expectedSRI)
				summary.record(test.observation)

				// When formatting a terminal indeterminate decision.
				line := summary.format(false, PublishIndeterminate)

				// Then only the categorical observation and final state are disclosed.
				for _, want := range []string{"observations=1 changes=0", `first="` + test.want + `"`, `final="` + test.want + `"`, "decision=indeterminate"} {
					if !strings.Contains(line, want) {
						t.Errorf("observation summary missing %q\n%s", want, line)
					}
				}
				if strings.Contains(line, "latest_changed=") {
					t.Errorf("unchanged observation emitted latest_changed\n%s", line)
				}
				for _, forbidden := range []string{"sha512-", "https://", packumentErr.Error(), attestationErr.Error()} {
					if strings.Contains(line, forbidden) {
						t.Errorf("observation summary contains raw value %q\n%s", forbidden, line)
					}
				}
			})
		}
	})

	t.Run("no change", func(t *testing.T) {
		// Given repeated identical observations.
		summary := newPublishObservationSummary(expectedSRI)
		summary.record(matching)
		summary.record(matching)

		// When formatting their shared final observation.
		line := summary.format(false, PublishCommittedAsExpected)

		// Then no changed observation is manufactured.
		if !strings.Contains(line, "observations=2 changes=0") || strings.Contains(line, "latest_changed=") {
			t.Fatalf("no-change summary = %s, want two observations and zero changes", line)
		}
	})

	t.Run("cancellation accumulation", func(t *testing.T) {
		// Given two retained observations before cancellation ends classification.
		summary := newPublishObservationSummary(expectedSRI)
		summary.record(registryObservation{packageExists: true})
		summary.record(registryObservation{packumentErr: packumentErr, attestationErr: attestationErr})

		// When formatting the cancellation's indeterminate final decision.
		line := summary.format(true, PublishIndeterminate)

		// Then cancellation cannot discard or add observations or disclose error text.
		for _, want := range []string{
			"observations=2 changes=1",
			`latest_changed="package=unknown version=unknown integrity=unknown attestations=unknown errors=both"`,
			`final="package=unknown version=unknown integrity=unknown attestations=unknown errors=both"`,
			"decision=indeterminate",
		} {
			if !strings.Contains(line, want) {
				t.Errorf("cancellation summary missing %q\n%s", want, line)
			}
		}
		for _, forbidden := range []string{packumentErr.Error(), attestationErr.Error(), "context canceled"} {
			if strings.Contains(line, forbidden) {
				t.Errorf("cancellation summary contains error text %q\n%s", forbidden, line)
			}
		}
	})
}

func requirePublishReadbackSummary(t *testing.T, output string, afterMutation bool) string {
	t.Helper()
	marker := "after_mutation=false"
	if afterMutation {
		marker = "after_mutation=true"
	}
	var matches []string
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "windlass operational: readback ") && strings.Contains(line, marker) {
			matches = append(matches, line)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("read-back summaries for %s = %d, want exactly one\n%s", marker, len(matches), output)
	}
	return matches[0]
}
