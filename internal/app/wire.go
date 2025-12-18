//go:build wireinject

package app

import (
	"github.com/mazrean/kessoku"
	"go.trai.ch/bob/internal/adapters/cas"
	"go.trai.ch/bob/internal/adapters/config"
	"go.trai.ch/bob/internal/adapters/fs"
	"go.trai.ch/bob/internal/adapters/logger"
	"go.trai.ch/bob/internal/adapters/nix"
	"go.trai.ch/bob/internal/adapters/shell"
	"go.trai.ch/bob/internal/core/ports"
	"go.trai.ch/bob/internal/engine/scheduler"
)

// AdapterSet groups all adapter providers with interface bindings.
var AdapterSet = kessoku.Set(
	// Logger (returns ports.Logger directly)
	kessoku.Provide(logger.New),

	// Config Loader
	kessoku.Bind[ports.ConfigLoader](kessoku.Provide(config.NewLoader)),

	// Shell Executor
	kessoku.Bind[ports.Executor](kessoku.Provide(shell.NewExecutor)),

	// FS Walker (concrete type, used by Hasher)
	kessoku.Provide(fs.NewWalker),

	// FS Resolver
	kessoku.Bind[ports.InputResolver](kessoku.Provide(fs.NewResolver)),

	// FS Hasher
	kessoku.Bind[ports.Hasher](kessoku.Provide(fs.NewHasher)),

	// CAS Store
	kessoku.Bind[ports.BuildInfoStore](kessoku.Provide(cas.NewStore)),

	// Nix Dependency Resolver
	kessoku.Bind[ports.DependencyResolver](kessoku.Provide(nix.NewResolver)),

	// Nix Package Manager
	kessoku.Bind[ports.PackageManager](kessoku.Provide(nix.NewManager)),

	// Nix Environment Factory
	kessoku.Bind[ports.EnvironmentFactory](kessoku.Provide(nix.NewEnvFactory)),
)

// EngineSet groups engine-layer providers.
var EngineSet = kessoku.Set(
	kessoku.Provide(scheduler.NewScheduler),
)

// AppSet groups application-layer providers.
var AppSet = kessoku.Set(
	kessoku.Provide(New),
	kessoku.Provide(NewComponents),
)

var _ = kessoku.Inject[*Components]("InitializeApp",
	AdapterSet,
	EngineSet,
	AppSet,
)

// InitializeApp is a stub for wire generation.
func InitializeApp() (*Components, error) {
	panic("wire")
}
