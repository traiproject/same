// Package app implements the application layer for bob.
package app

import (
	"go.trai.ch/bob/internal/adapters/config" //nolint:depguard // Builders need to wire concrete adapters
	"go.trai.ch/bob/internal/core/ports"
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
