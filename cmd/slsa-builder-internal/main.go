package main

import (
	"context"
	"os"

	"github.com/windlasstech/slsa-builder/internal/command"
)

func main() {
	result := command.NewDispatcher().Dispatch(context.Background(), os.Args[1:], os.Stdout)
	os.Exit(result.ExitCode)
}
