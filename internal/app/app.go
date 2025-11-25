// Package app implements the application layer for bob.
package app

import (
	"context"
	"fmt"
	"runtime"

	"go.trai.ch/bob/internal/core/ports"
	"go.trai.ch/bob/internal/engine/scheduler"
)

// App represents the main application logic.
type App struct {
	configLoader ports.ConfigLoader
	scheduler    *scheduler.Scheduler
}

// New creates a new App instance.
func New(loader ports.ConfigLoader, sched *scheduler.Scheduler) *App {
	return &App{
		configLoader: loader,
		scheduler:    sched,
	}
}

// Run executes the build process for the specified targets.
func (a *App) Run(ctx context.Context, targetNames []string) error {
	// 1. Load the graph
	graph, err := a.configLoader.Load(".")
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// 2. Validate targets
	if len(targetNames) == 0 {
		return fmt.Errorf("no targets specified")
	}

	// 3. Run the scheduler
	if err := a.scheduler.Run(ctx, graph, targetNames, runtime.NumCPU()); err != nil {
		return fmt.Errorf("build execution failed: %w", err)
	}

	return nil
}
