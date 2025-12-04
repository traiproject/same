// Package app implements the application layer for bob.
package app

import (
	"go.trai.ch/bob/internal/adapters/config"
	"go.trai.ch/bob/internal/core/ports"
)

// Components contains all the initialized application components.
// This struct provides controlled access to components needed by the CLI layer.
type Components struct {
	App          *App
	Logger       ports.Logger
	configLoader *config.Loader
}

// NewComponents creates a new Components struct from dependencies.
// This is used by the kessoku-generated injector.
func NewComponents(app *App, logger ports.Logger, loader *config.Loader) *Components {
	return &Components{
		App:          app,
		Logger:       logger,
		configLoader: loader,
	}
}

// NewApp creates and configures a new App instance with all required dependencies.
// It uses the kessoku-generated InitializeApp function in wire_band.go.
// Returns the configured Components and any initialization error.
func NewApp() (*Components, error) {
	return InitializeApp()
}
