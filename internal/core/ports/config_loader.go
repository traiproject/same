package ports

import "go.trai.ch/bob/internal/core/domain"

// ConfigLoader defines the interface for loading the build configuration.
//
//go:generate mockgen -source=config_loader.go -destination=mocks/mock_config_loader.go -package=mocks
type ConfigLoader interface {
	// Load reads the configuration from the given working directory and returns the task graph.
	Load(cwd string) (*domain.Graph, error)
}
