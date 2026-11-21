package command_test

import (
	"bytes"
	"strings"
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
		{"unsupported api base", []string{"--mode", "plan", "--api-base", "http://localhost"}, 2},
		{"positionals", []string{"extra"}, 2},
		{"unknown mode", []string{"--mode", "unknown", "--event", "event.json", "--trusted-root", ".", "--trusted-sha", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "--output", "output"}, 2},
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
	// Given a valid dummy token, NewRemote succeeds without performing a request.
	t.Setenv("GITHUB_TOKEN", "ghp_valid_dummy_token_for_local_gate_test")
	t.Setenv("DEPENDABOT_COMPANION_APPLY_ENABLED", "false")
	args := []string{"dependabot-companion", "--mode", "apply", "--event", "missing-event.json", "--trusted-root", ".", "--trusted-sha", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "--output", "output"}
	var out bytes.Buffer
	result := command.NewDispatcher(command.NewDependabotCompanionCommand()).Dispatch(t.Context(), args, &out)
	if result.ExitCode != command.ExitCodeInvocationFailure || !strings.Contains(out.String(), "apply disabled") {
		t.Fatalf("valid-token apply must report explicit apply-disabled gate, exit=%d output=%s", result.ExitCode, &out)
	}
	if strings.Contains(out.String(), "ghp_valid_dummy") || strings.Contains(out.String(), "event") || strings.Contains(out.String(), "GitHub") {
		t.Fatalf("gate ran after input or remote work, or exposed token: %s", &out)
	}
}

func TestDependabotPlanRejectsEmptyToken(t *testing.T) {
	// Given an empty token, the plan mode fails while constructing the remote client.
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("DEPENDABOT_COMPANION_APPLY_ENABLED", "false")
	args := []string{"dependabot-companion", "--mode", "plan", "--event", "missing-event.json", "--trusted-root", ".", "--trusted-sha", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "--output", "output"}
	var out bytes.Buffer
	result := command.NewDispatcher(command.NewDependabotCompanionCommand()).Dispatch(t.Context(), args, &out)
	if result.ExitCode != command.ExitCodeInvocationFailure {
		t.Fatalf("empty-token plan exit=%d output=%s", result.ExitCode, &out)
	}
	if strings.Contains(out.String(), "apply disabled") || strings.Contains(out.String(), "event") {
		t.Fatalf("plan failure did not remain separate from apply gate/input processing: %s", &out)
	}
}

func TestDependabotCompanionSnapshotRequirements(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "ghp_valid_dummy_token_for_local_gate_test")
	t.Setenv("DEPENDABOT_COMPANION_APPLY_ENABLED", "false")
	for _, test := range []struct {
		name     string
		mode     string
		snapshot string
	}{
		{name: "dry-run requires snapshot", mode: "dry-run"},
		{name: "plan rejects snapshot", mode: "plan", snapshot: "local"},
		{name: "apply rejects snapshot", mode: "apply", snapshot: "local"},
	} {
		t.Run(test.name, func(t *testing.T) {
			args := []string{"dependabot-companion", "--mode", test.mode, "--event", "event.json", "--trusted-root", ".", "--trusted-sha", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "--output", "output"}
			if test.snapshot != "" {
				args = append(args, "--snapshot", test.snapshot)
			}
			var out bytes.Buffer
			result := command.NewDispatcher(command.NewDependabotCompanionCommand()).Dispatch(t.Context(), args, &out)
			if result.ExitCode != command.ExitCodeInvocationFailure || !strings.Contains(out.String(), "input") {
				t.Fatalf("exit=%d output=%s, want invocation failure identifying input rejection", result.ExitCode, &out)
			}
		})
	}
}
