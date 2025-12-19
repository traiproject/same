// Package app implements the application layer for bob.
package app

import (
	"go.trai.ch/bob/internal/adapters/cas"    //nolint:depguard // Builders need to wire concrete adapters
	"go.trai.ch/bob/internal/adapters/config" //nolint:depguard // Builders need to wire concrete adapters
	"go.trai.ch/bob/internal/adapters/fs"     //nolint:depguard // Builders need to wire concrete adapters
	"go.trai.ch/bob/internal/adapters/logger" //nolint:depguard // Builders need to wire concrete adapters
	"go.trai.ch/bob/internal/adapters/nix"    //nolint:depguard // Builders need to wire concrete adapters
	"go.trai.ch/bob/internal/adapters/shell"  //nolint:depguard // Builders need to wire concrete adapters
	"go.trai.ch/bob/internal/core/ports"
	"go.trai.ch/bob/internal/engine/scheduler"
)

// Components contains all the initialized application components.
// This struct provides controlled access to components needed by the CLI layer.
type Components struct {
	App          *App
	Logger       ports.Logger
	configLoader ports.ConfigLoader
}

// NewComponents creates a new Components struct from dependencies.
// This is used by the kessoku-generated injector.
func NewComponents(app *App, log ports.Logger, loader *config.Loader) *Components {
	return &Components{
		App:          app,
		Logger:       log,
		configLoader: loader,
	}
}

// NewApp creates and configures a new App instance with all required dependencies.
// It uses the kessoku-generated InitializeApp function in wire_band.go.
// Returns the configured Components and any initialization error.
// NewApp creates and configures a new App instance with all required dependencies.
// This manually wires the application components, replacing the previous Kessoku-generated wiring.
func NewApp() (*Components, error) {
	// 1. Core Adapters
	loggerAdapter := logger.New()

	walker := fs.NewWalker()
	hasher := fs.NewHasher(walker)

	configLoader := config.NewLoader(loggerAdapter)

	shellExecutor := shell.NewExecutor(loggerAdapter)

	fsResolver := fs.NewResolver()

	casStore, err := cas.NewStore()
	if err != nil {
		return nil, err
	}

	nixResolver, err := nix.NewResolver()
	if err != nil {
		return nil, err
	}

	nixEnvFactory := nix.NewEnvFactory(nixResolver)

	// 2. Engine
	sched := scheduler.NewScheduler(
		shellExecutor,
		casStore,
		hasher,
		fsResolver,
		loggerAdapter,
		nixEnvFactory,
	)

	// 3. Application
	app := New(configLoader, sched)

	// 4. Components
	return NewComponents(app, loggerAdapter, configLoader), nil
}
