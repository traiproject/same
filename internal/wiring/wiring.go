// Package wiring registers all Graft nodes for the application.
package wiring

import (
	// Register adapter nodes.
	_ "go.trai.ch/bob/internal/adapters/cas"
	_ "go.trai.ch/bob/internal/adapters/config"
	_ "go.trai.ch/bob/internal/adapters/fs"
	_ "go.trai.ch/bob/internal/adapters/logger"
	_ "go.trai.ch/bob/internal/adapters/nix"
	_ "go.trai.ch/bob/internal/adapters/shell"
	// Register app and engine nodes.
	_ "go.trai.ch/bob/internal/app"
	_ "go.trai.ch/bob/internal/engine/scheduler"
)
