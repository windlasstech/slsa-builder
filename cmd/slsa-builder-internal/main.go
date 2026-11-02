package main

import (
	"context"
	"io"
	"os"
	"syscall"

	"github.com/windlasstech/slsa-builder/internal/command"
	"github.com/windlasstech/slsa-builder/internal/npmprofile"
)

func main() {
	diagnostics, closeDiagnostics := operationalDebugStderr()
	ctx := npmprofile.WithOperationalDebugOutput(context.Background(), diagnostics)
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
	).Dispatch(ctx, os.Args[1:], os.Stdout)
	closeDiagnostics()
	os.Exit(result.ExitCode)
}

// operationalDebugStderr duplicates fd 2 so a broken diagnostic pipe returns EPIPE instead of
// triggering Go's special SIGPIPE behavior for writes to stdout or stderr.
func operationalDebugStderr() (io.Writer, func()) {
	descriptor, err := syscall.Dup(int(os.Stderr.Fd()))
	if err != nil {
		return io.Discard, func() {}
	}
	file := os.NewFile(uintptr(descriptor), os.Stderr.Name())
	return file, func() {
		if err := file.Close(); err != nil {
			return
		}
	}
}
