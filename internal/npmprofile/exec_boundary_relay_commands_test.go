package npmprofile

import (
	"strings"
	"testing"
)

func TestOperationalStderrRelayRunnerCommands(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, input, secret, want string }{
		{"set-output", "prefix##[set-output name=x]y", "", "prefix##set-output name=x]y"},
		{"add-mask", "##[add-mask]foo", "", "##add-mask]foo"},
		{"error", "prefix ##[error]bad", "", "prefix ##error]bad"},
		{"stop-commands", "##[stop-commands]tok", "", "##stop-commands]tok"},
		{"near misses", "## [ #[] #[ ##x[", "", "## [ #[] #[ ##x["},
		{"modern unchanged", "::error::bad middle ::stop-commands::tok", "", "::error::bad middle ::stop-commands::tok"},
		{"redaction composed", "prefix##secret", "secret", "prefix##REDACTED]"},
		{"stripping composed", "#\x00#\u202e[error]bad", "", "##error]bad"},
		{"repeated brackets", "##[[[error]bad", "", "##error]bad"},
		{"hash suffix", "###\r[error]bad", "", "###error]bad"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			for chunk := 1; chunk <= len(test.input)+1; chunk++ {
				got := relayForTest(t, test.input, []string{test.secret}, chunk)
				want := "windlass operational: operation=npm stderr: " + test.want + "\n"
				if got != want {
					t.Fatalf("chunk %d: got %q, want %q", chunk, got, want)
				}
			}
		})
	}
}

func TestOperationalStderrRelayNeutralizationBeforeBounds(t *testing.T) {
	t.Parallel()
	input := strings.Repeat("x", 65534) + "##[[["
	want := "windlass operational: operation=npm stderr: " + strings.Repeat("x", 65534) + "##\n"
	if got := relayForTest(t, input, nil, 1); got != want {
		t.Fatal("neutralized bytes consumed payload budget")
	}
	got := relayForTest(t, "##\n[error]not-a-command", nil, 1)
	if !strings.Contains(got, "stderr: [error]not-a-command\n") {
		t.Fatal("neutralization crossed a line boundary")
	}
}
