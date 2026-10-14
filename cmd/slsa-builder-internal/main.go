package main

import (
	"context"
	"os"

	"github.com/windlasstech/slsa-builder/internal/command"
)

func main() {
	result := command.NewDispatcher(
		command.NewFixtureCheckCommand(),
		command.NewNPMProfileBuildCommand(),
		command.NewNPMProfileProvisionNPMCommand(),
		command.NewNPMProfileProvisionNodeCommand(),
		command.NewNPMProfilePublishCommand(),
		command.NewNPMProfileReportCommand(),
		command.NewNPMProfileSignCommand(),
		command.NewNPMProfileSelectCommand(),
		command.NewNPMProfileSourceRefCommand(),
		command.NewNPMProfileToolchainAssertCommand(),
		command.NewPinCheckCommand(),
		command.NewVerifyHandoffCommand(),
		command.NewVerifyAttestationCommand(),
		command.NewWorkflowCheckCommand(),
	).Dispatch(context.Background(), os.Args[1:], os.Stdout)
	os.Exit(result.ExitCode)
}
