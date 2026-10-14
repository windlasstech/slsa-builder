package command

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
)

type stagedWriter struct {
	remaining int
	err       error
}

func (writer *stagedWriter) Write(data []byte) (int, error) {
	if writer.remaining == 0 {
		return 0, writer.err
	}
	writer.remaining--
	return len(data), nil
}

func TestRegisteredAdapterNames(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		command Command
		want    string
	}{
		{NewFixtureCheckCommand(), "fixture-check"},
		{NewNPMProfileBuildCommand(), "npm-profile-build"},
		{NewNPMProfileProvisionNPMCommand(), "npm-profile-provision-npm"},
		{NewNPMProfileProvisionNodeCommand(), "npm-profile-provision-node"},
		{NewNPMProfilePublishCommand(), "npm-profile-publish"},
		{NewNPMProfileReportCommand(), "npm-profile-report"},
		{NewNPMProfileSelectCommand(), "npm-profile-select"},
		{NewNPMProfileSignCommand(), "npm-profile-sign"},
		{NewNPMProfileSourceRefCommand(), "npm-profile-source-ref"},
		{NewNPMProfileToolchainAssertCommand(), "npm-profile-toolchain-assert"},
		{NewPinCheckCommand(), "pin-check"},
		{NewVerifyAttestationCommand(), "verify-attestation"},
		{NewVerifyHandoffCommand(), "verify-handoff"},
		{NewWorkflowCheckCommand(), "workflow-check"},
	} {
		t.Run(test.want, func(t *testing.T) {
			t.Parallel()
			if got := test.command.Name(); got != test.want {
				t.Fatalf("Name() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestUsageOutput(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name     string
		commands map[string]Command
		want     string
	}{
		{"empty", nil, "Usage: slsa-builder-internal [subcommand] [args...]\nNo subcommands registered.\n"},
		{"sorted", map[string]Command{"workflow-check": NewWorkflowCheckCommand(), "fixture-check": NewFixtureCheckCommand()}, "Usage: slsa-builder-internal [subcommand] [args...]\nRegistered subcommands:\n  fixture-check\n  workflow-check\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var output bytes.Buffer
			if err := writeUsage(&output, test.commands); err != nil {
				t.Fatal(err)
			}
			if output.String() != test.want {
				t.Fatalf("usage = %q, want %q", output.String(), test.want)
			}
		})
	}
}

func TestUsagePropagatesEachWriteFailure(t *testing.T) {
	t.Parallel()
	for _, populated := range []bool{false, true} {
		writes := 2
		commands := map[string]Command{}
		if populated {
			commands["fixture-check"] = NewFixtureCheckCommand()
			writes = 3
		}
		for stage := range writes {
			t.Run(fmt.Sprintf("populated=%t/stage=%d", populated, stage), func(t *testing.T) {
				t.Parallel()
				cause := errors.New("usage sink failed")
				err := writeUsage(&stagedWriter{remaining: stage, err: cause}, commands)
				if !errors.Is(err, cause) {
					t.Fatalf("writeUsage() = %v, want writer failure", err)
				}
			})
		}
	}
}

func TestDispatcherHelpAndWriteFailures(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{nil, {"--help"}, {"-h"}, {"unknown"}, {"fixture-check"}} {
		t.Run(fmt.Sprint(args), func(t *testing.T) {
			t.Parallel()
			dispatcher := NewDispatcher(NewFixtureCheckCommand())
			result := dispatcher.Dispatch(t.Context(), args, &stagedWriter{err: errors.New("sink failed")})
			if result.ExitCode != ExitCodeInvocationFailure {
				t.Fatalf("Dispatch() exit = %d, want 2", result.ExitCode)
			}
		})
	}
	t.Run("successful help", func(t *testing.T) {
		t.Parallel()
		var output bytes.Buffer
		result := NewDispatcher().Dispatch(t.Context(), nil, &output)
		if result.ExitCode != 0 || output.String() != "Usage: slsa-builder-internal [subcommand] [args...]\nNo subcommands registered.\n" {
			t.Fatalf("help = %#v, %q", result, output.String())
		}
	})
}
