package command

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"strings"
	"testing"
)

func TestAdapterInvocationGuards(t *testing.T) {
	t.Parallel()
	for _, command := range []Command{NewFixtureCheckCommand(), NewNPMProfileBuildCommand(), NewNPMProfilePublishCommand(), NewNPMProfileReportCommand(), NewNPMProfileSelectCommand(), NewNPMProfileSignCommand(), NewNPMProfileSourceRefCommand(), NewVerifyAttestationCommand(), NewVerifyHandoffCommand(), NewWorkflowCheckCommand()} {
		t.Run(command.Name(), func(t *testing.T) {
			t.Parallel()
			for _, test := range []struct {
				name string
				args []string
			}{
				{"missing flags", nil}, {"unknown flag", []string{"--unknown-flag"}},
			} {
				t.Run(test.name, func(t *testing.T) {
					t.Parallel()
					var output bytes.Buffer
					err := command.Execute(t.Context(), test.args, &output)
					if err == nil || errors.Is(err, ErrVerificationFailure) || errors.Is(err, ErrInvocationFailure) || output.Len() != 0 {
						t.Fatalf("invalid invocation = %v, output %q; want raw error and no output", err, output.String())
					}
				})
			}
			t.Run("help", func(t *testing.T) {
				t.Parallel()
				var output bytes.Buffer
				err := command.Execute(t.Context(), []string{"--help"}, &output)
				if command.Name() == "npm-profile-publish" || command.Name() == "npm-profile-report" {
					if !errors.Is(err, flag.ErrHelp) || output.Len() != 0 {
						t.Fatalf("flag help = %v, %q", err, output.String())
					}
					return
				}
				if err != nil || !strings.Contains(output.String(), "Usage: slsa-builder-internal "+command.Name()) {
					t.Fatalf("help = %v, %q", err, output.String())
				}
			})
			if command.Name() != "npm-profile-publish" {
				t.Run("cancelled", func(t *testing.T) {
					t.Parallel()
					ctx, cancel := context.WithCancel(t.Context())
					cancel()
					var output bytes.Buffer
					if err := command.Execute(ctx, nil, &output); !errors.Is(err, context.Canceled) || output.Len() != 0 {
						t.Fatalf("cancelled Execute = %v, output %q", err, output.String())
					}
				})
			}
		})
	}
}
