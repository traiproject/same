// Package main is the entry point for the same build tool.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/grindlemire/graft"
	"go.trai.ch/same/cmd/same/commands"
	"go.trai.ch/same/internal/app"
	"go.trai.ch/same/internal/core/domain"
	_ "go.trai.ch/same/internal/wiring"
)

// ComponentProvider is a function that returns the application components.
type ComponentProvider func(context.Context) (*app.Components, func(), error)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stderr, func(ctx context.Context) (*app.Components, func(), error) {
		c, _, err := graft.ExecuteFor[*app.Components](ctx)
		return c, func() {}, err
	}))
}

func run(
	ctx context.Context,
	args []string,
	stderr io.Writer,
	provider ComponentProvider,
	opts ...func(*app.App),
) int {
	// 0. Context with signal handling
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// 1. Initialize application components
	components, _, err := provider(ctx)
	if err != nil {
		// Logger is not available yet if initialization failed
		// Write directly to stderr passed in
		_, _ = fmt.Fprintln(stderr, "Error: "+err.Error())
		return 1
	}

	// Apply options
	for _, opt := range opts {
		opt(components.App)
	}

	// 2. Interface - CLI
	cli := commands.New(components.App)
	cli.SetArgs(args)
	cli.SetOutput(os.Stdout, stderr)

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
