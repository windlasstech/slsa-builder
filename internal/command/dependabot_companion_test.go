package command_test

import (
	"bytes"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/command"
)

func TestDependabotCompanionFlags(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		args []string
		code int
	}{
		{"help", []string{"--help"}, 0},
		{"missing", nil, 2},
		{"apply missing flags", []string{"--mode", "apply"}, 2},
		{"plan missing flags", []string{"--mode", "plan"}, 2},
		{"unknown", []string{"--unknown"}, 2},
		{"positionals", []string{"extra"}, 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var out bytes.Buffer
			result := command.NewDispatcher(command.NewDependabotCompanionCommand()).Dispatch(t.Context(), append([]string{"dependabot-companion"}, test.args...), &out)
			if result.ExitCode != test.code {
				t.Fatalf("exit=%d output=%s", result.ExitCode, &out)
			}
		})
	}
}

func TestDependabotRemoteInvocationGates(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("DEPENDABOT_COMPANION_APPLY_ENABLED", "false")
	for _, mode := range []string{"plan", "apply"} {
		for _, extra := range [][]string{nil, {"--snapshot", "local"}, {"--api-base", "http://localhost"}} {
			args := []string{"dependabot-companion", "--mode", mode, "--event", "event.json", "--trusted-root", ".", "--trusted-sha", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "--output", "output"}
			var out bytes.Buffer
			result := command.NewDispatcher(command.NewDependabotCompanionCommand()).Dispatch(t.Context(), append(args, extra...), &out)
			if result.ExitCode != 2 {
				t.Fatalf("mode=%s extra=%v exit=%d", mode, extra, result.ExitCode)
			}
		}
	}
}
