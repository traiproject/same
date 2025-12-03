// Package app implements the application layer for bob.
package app

import (
	"go.trai.ch/bob/internal/adapters/cas"
	"go.trai.ch/bob/internal/adapters/config"
	"go.trai.ch/bob/internal/adapters/fs"
	"go.trai.ch/bob/internal/adapters/logger"
	"go.trai.ch/bob/internal/adapters/shell"
	"go.trai.ch/bob/internal/core/ports"
	"go.trai.ch/bob/internal/engine/scheduler"
	"go.trai.ch/zerr"
)

// Components contains all the initialized application components.
// This struct provides controlled access to components needed by the CLI layer.
type Components struct {
	App          *App
	Logger       ports.Logger
	configLoader *config.FileConfigLoader
}

// SetConfigPath updates the configuration file path dynamically.
// This is used by the CLI layer to respond to command-line flags.
func (c *Components) SetConfigPath(path string) {
	c.configLoader.Filename = path
}

// NewApp creates and configures a new App instance with all required dependencies.
// It instantiates all necessary adapters and wires them together.
// Returns the configured Components and any initialization error.
func NewApp(configPath, stateDir string) (*Components, error) {
	// 1. Infrastructure - Logger (needed early for error reporting)
	log := logger.New()

	// 2. Infrastructure - Config Loader
	configLoader := &config.FileConfigLoader{Filename: configPath}

	// 3. Infrastructure - Execution and File System
	executor := shell.NewExecutor(log)
	walker := fs.NewWalker()
	resolver := fs.NewResolver()
	hasher := fs.NewHasher(walker)

	// 4. Infrastructure - Content-Addressed Store
	store, err := cas.NewStore(stateDir)
	if err != nil {
		return nil, zerr.Wrap(err, "failed to initialize build info store")
	}

	// 5. Engine - Scheduler
	sched := scheduler.NewScheduler(executor, store, hasher, resolver, log)

	// 6. Application - Wire everything together
	application := New(configLoader, sched)

	return &Components{
		App:          application,
		Logger:       log,
		configLoader: configLoader,
	}, nil
}
