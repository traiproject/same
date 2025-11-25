// Package main is the entry point for the bob CLI.
package main

import (
	"context"
	"fmt"
	"os"

	"go.trai.ch/bob/internal/adapters/cas"
	"go.trai.ch/bob/internal/adapters/config"
	"go.trai.ch/bob/internal/adapters/shell"
	"go.trai.ch/bob/internal/app"
	"go.trai.ch/bob/internal/engine/scheduler"
)

func main() {
	if err := run(); err != nil {
		// zerr prints a pretty error report with stack trace and metadata when using %+v
		_, _ = fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()

	// Initialize adapters
	configLoader := &config.FileConfigLoader{Filename: "bob.yaml"}

	logger := &stdLogger{}
	executor := shell.NewExecutor(logger)

	// Initialize BuildInfoStore
	// Note: Currently not used by Scheduler, but initialized as requested.
	// This might be used in future iterations for caching.
	_, err := cas.NewStore("bob_state.json")
	if err != nil {
		return err
	}

	// Initialize engine
	sched := scheduler.NewScheduler(executor)

	// Initialize app
	application := app.New(configLoader, sched)

	// Run build
	// TODO: Parse targets from args
	return application.Run(ctx, nil)
}

type stdLogger struct{}

func (l *stdLogger) Info(msg string) {
	_, _ = fmt.Fprintln(os.Stdout, msg)
}

func (l *stdLogger) Error(msg string) {
	_, _ = fmt.Fprintln(os.Stderr, msg)
}
