// Package wiring registers all Graft nodes for the application.
package wiring

import (
	// Register adapter nodes.
	_ "go.trai.ch/same/internal/adapters/cas"
	_ "go.trai.ch/same/internal/adapters/config"
	_ "go.trai.ch/same/internal/adapters/daemon"
	_ "go.trai.ch/same/internal/adapters/fs"
	_ "go.trai.ch/same/internal/adapters/logger"
	_ "go.trai.ch/same/internal/adapters/nix"
	_ "go.trai.ch/same/internal/adapters/shell"
	// Register app and engine nodes.
	_ "go.trai.ch/same/internal/app"
	_ "go.trai.ch/same/internal/engine/scheduler"
)
