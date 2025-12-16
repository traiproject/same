// Package app implements the application layer for bob.
package app

//go:generate sh -c "GOFLAGS='-tags=wireinject' go run github.com/mazrean/kessoku/cmd/kessoku wire.go"

import (
	"context"
	"runtime"

	"go.trai.ch/bob/internal/core/domain"
	"go.trai.ch/bob/internal/core/ports"
	"go.trai.ch/bob/internal/engine/scheduler"
	"go.trai.ch/zerr"
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
func (a *App) Run(ctx context.Context, targetNames []string, force bool) error {
	// 1. Load the graph
	graph, err := a.configLoader.Load(".")
	if err != nil {
		return zerr.Wrap(err, "failed to load configuration")
	}

	// 2. Validate targets
	if len(targetNames) == 0 {
		return domain.ErrNoTargetsSpecified
	}

	// 3. Run the scheduler
	if err := a.scheduler.Run(ctx, graph, targetNames, runtime.NumCPU(), force); err != nil {
		return zerr.Wrap(err, "build execution failed")
	}

	return nil
}
