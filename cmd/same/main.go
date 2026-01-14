// Package main is the entry point for the bob build tool.
package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/grindlemire/graft"
	"go.trai.ch/same/cmd/same/commands"
	"go.trai.ch/same/internal/app"
	"go.trai.ch/same/internal/core/domain"
	_ "go.trai.ch/same/internal/wiring"
)

func main() {
	os.Exit(run())
}

func run(opts ...func(*app.App)) int {
	// 0. Context with signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// 1. Initialize application components
	components, _, err := graft.ExecuteFor[*app.Components](ctx)
	if err != nil {
		// Logger is not available yet if initialization failed
		// Write directly to stderr
		_, _ = os.Stderr.WriteString("Error: " + err.Error() + "\n")
		return 1
	}

	// Apply options
	for _, opt := range opts {
		opt(components.App)
	}

	// 2. Interface - CLI
	cli := commands.New(components.App)

	// 4. Execution
	if err := cli.Execute(ctx); err != nil {
		if errors.Is(err, domain.ErrBuildExecutionFailed) {
			return 1
		}
		components.Logger.Error(err)
		return 1
	}
	return 0
}
