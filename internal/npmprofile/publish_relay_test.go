package npmprofile

import (
	"bytes"
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
