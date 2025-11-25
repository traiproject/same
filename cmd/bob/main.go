// Package main is the entry point for the bob CLI.
package main

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"go.trai.ch/bob/internal/adapters/cas"
	"go.trai.ch/bob/internal/adapters/config"
	"go.trai.ch/bob/internal/adapters/fs"
	"go.trai.ch/bob/internal/adapters/shell"
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
	graph, err := config.Load("bob.yaml")
	if err != nil {
		return err
	}

	logger := &stdLogger{}
	executor := shell.NewExecutor(logger)

	// Initialize Hasher
	walker := fs.NewWalker()
	hasher := fs.NewHasher(walker)

	// Initialize BuildInfoStore
	// Note: Currently not used by Scheduler, but initialized as requested.
	// This might be used in future iterations for caching.
	store, err := cas.NewStore("bob_state.json")
	if err != nil {
		return err
	}

	// Initialize engine
	sched, err := scheduler.NewScheduler(graph, executor, hasher, store)
	if err != nil {
		return err
	}

	// Run scheduler
	if err := sched.Run(ctx, runtime.NumCPU()); err != nil {
		return err
	}

	return nil
}

type stdLogger struct{}

func (l *stdLogger) Info(msg string) {
	_, _ = fmt.Fprintln(os.Stdout, msg)
}

func (l *stdLogger) Error(msg string) {
	_, _ = fmt.Fprintln(os.Stderr, msg)
}
