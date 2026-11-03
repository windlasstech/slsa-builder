package npmprofile

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestPublishRelayRedactsOIDCMaterial(t *testing.T) {
	fixture := newPublishFixture(t, publishFixtureOptions{initialVersionAbsent: true, npmExitCode: 1,
		npmError: "unknown registry rejection https://registry.npmjs.org/package /public/path\nambient-canary identity-canary short-lived-token-must-not-reach-npm-env"})
	fixture.request.OIDCExchange.GitHubIdentityToken = newSecretToken("identity-canary")
	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", "ambient-canary")
	t.Setenv("NPM_CONFIG_LOGLEVEL", "silly")
	t.Setenv("npm_config_loglevel", "verbose")
	var log bytes.Buffer
	result, err := Publish(WithOperationalDebugOutput(t.Context(), &log), fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(log.String(), "stderr: unknown registry rejection https://registry.npmjs.org/package /public/path") || !strings.Contains(log.String(), "stderr: [REDACTED] [REDACTED] [REDACTED]") {
		t.Fatal("missing public text or redacted OIDC material")
	}
	encoded, err := result.Report.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"ambient-canary", "identity-canary", "short-lived-token-must-not-reach-npm-env"} {
		if strings.Contains(log.String(), secret) || strings.Contains(string(encoded), secret) {
			t.Fatal("secret leaked")
		}
	}
	fixture.requireInvocations(t, fixture.expectedMutationInvocations())
	for _, arg := range fixture.expectedArgv() {
		if strings.Contains(strings.ToLower(arg), "loglevel") {
			t.Fatal("publish elevates loglevel")
		}
	}
	for _, entry := range publishEnvironment([]string{"NPM_CONFIG_LOGLEVEL=silly", "npm_config_loglevel=verbose"}, "home", "user", "global") {
		if strings.Contains(strings.ToLower(entry), "loglevel") {
			t.Fatal("publish environment elevates loglevel")
		}
	}
}

func TestPublishRelayAndFamilyEnrichmentAreIndependent(t *testing.T) {
	for _, test := range []struct{ name, message, family string }{
		{"known", frameNPMFamilyMessage(predicateFamilyMessage), "predicate-field-mismatch"},
		{"unknown", "npm error 422 Error verifying sigstore provenance bundle: Novel registry rejection", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given: a failed npm process followed by successful authoritative read-back.
			options := publishFixtureOptions{initialVersionAbsent: true, npmExitCode: 1,
				npmError: "npm error code E422\n" + test.message}
			fixture := newPublishFixture(t, options)
			var log bytes.Buffer
			// When: Publish observes the failed process with operational diagnostics enabled.
			result, err := Publish(WithOperationalDebugOutput(t.Context(), &log), fixture.request)
			if err != nil {
				t.Fatal(err)
			}
			// Then: known and novel text both relay; only known text enriches the summary.
			want := npmFailureSummaryPrefix
			if test.family != "" {
				want += " family=" + test.family
			}
			want += " code=E422 http_status=422\n"
			if strings.Count(log.String(), want) != 1 {
				t.Fatalf("missing summary %q in %s", want, log.String())
			}
			if test.family == "" && strings.Contains(log.String(), " family=") {
				t.Fatal("unknown message gained a family")
			}
			for line := range strings.SplitSeq(test.message, "\n") {
				if !strings.Contains(log.String(), "operation=npm stderr: "+line+"\n") {
					t.Fatal("relay omitted child message")
				}
			}
			if result.State != PublishCommittedAsExpected || !result.MutationAttempted || fixture.packumentRequests != 2 || fixture.attestationRequests != 2 || fixture.clock.sleeps != 0 {
				t.Fatal("family enrichment changed publication or observation behavior")
			}
			fixture.requireInvocations(t, fixture.expectedMutationInvocations())
			encoded, err := result.Report.CanonicalJSON()
			if err != nil {
				t.Fatal(err)
			}
			controlFixture := newPublishFixture(t, options)
			control, err := Publish(WithOperationalDebugOutput(t.Context(), io.Discard), controlFixture.request)
			if err != nil {
				t.Fatal(err)
			}
			controlBytes, err := control.Report.CanonicalJSON()
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(encoded, controlBytes) {
				t.Fatal("enrichment changed canonical report")
			}
		})
	}
}
