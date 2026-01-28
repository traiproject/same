package ports

import "go.trai.ch/same/internal/core/domain"

// ConfigLoader defines the interface for loading the build configuration.
//
//go:generate mockgen -source=config_loader.go -destination=mocks/mock_config_loader.go -package=mocks
type ConfigLoader interface {
	// Load reads the configuration from the given working directory and returns the task graph.
	Load(cwd string) (*domain.Graph, error)

	// DiscoverConfigPaths finds configuration file paths and their modification times.
	// Returns a map of config file paths to their mtime in UnixNano.
	DiscoverConfigPaths(cwd string) (map[string]int64, error)

	// DiscoverRoot walks up from cwd to find the workspace root.
	// Returns the directory containing same.work.yaml or same.yaml.
	DiscoverRoot(cwd string) (string, error)
}
