// Package main is the entry point for the bob build tool.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"go.trai.ch/bob/cmd/bob/commands"
	"go.trai.ch/bob/internal/app"
)

func main() {
	os.Exit(run())
}

func run() int {
	// 0. Context with signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// 1. Initialize application components
	components, err := app.NewApp()
	if err != nil {
		// Logger is not available yet if initialization failed
		// Write directly to stderr
		_, _ = os.Stderr.WriteString("Error: " + err.Error() + "\n")
		return 1
	}

	// 2. Interface - CLI
	cli := commands.New(components.App)

	// 4. Execution
	if err := cli.Execute(ctx); err != nil {
		components.Logger.Error(err)
		return 1
	}
	return 0
}
