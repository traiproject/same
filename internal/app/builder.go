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
	ConfigLoader ports.ConfigLoader
	Executor     ports.Executor
	Store        ports.BuildInfoStore
	Hasher       ports.Hasher
	Resolver     ports.InputResolver
	EnvFactory   ports.EnvironmentFactory
}

// NewComponents creates a new Components struct from dependencies.
// This is used by the kessoku-generated injector.
func NewComponents(
	app *App,
	log ports.Logger,
	loader *config.Loader,
	executor ports.Executor,
	store ports.BuildInfoStore,
	hasher ports.Hasher,
	resolver ports.InputResolver,
	envFactory ports.EnvironmentFactory,
) *Components {
	return &Components{
		App:          app,
		Logger:       log,
		ConfigLoader: loader,
		Executor:     executor,
		Store:        store,
		Hasher:       hasher,
		Resolver:     resolver,
		EnvFactory:   envFactory,
	}
}
