package nix

import "time"

// Parse the JSON output to extract the store path.
type buildResults []struct {
	DrvPath string            `json:"drvPath"`
	Outputs map[string]string `json:"outputs"`
}

// cacheEntry represents a cached resolution result with per-system commit hashes.
type cacheEntry struct {
	Alias     string                 `json:"alias"`
	Version   string                 `json:"version"`
	Systems   map[string]SystemCache `json:"systems"`
	Timestamp time.Time              `json:"timestamp"`
}

// SystemCache represents cached data for a specific system architecture.
type SystemCache struct {
	FlakeInstallable FlakeInstallable `json:"flake_installable"`
	Outputs          []Output         `json:"outputs"`
}

// nixHubResponse represents the complete API response from NixHub v2/resolve.
type nixHubResponse struct {
	Name    string                    `json:"name"`
	Version string                    `json:"version"`
	Summary string                    `json:"summary"`
	Systems map[string]SystemResponse `json:"systems"`
}

// SystemResponse represents package information for a specific system architecture.
type SystemResponse struct {
	FlakeInstallable FlakeInstallable `json:"flake_installable"`
	LastUpdated      string           `json:"last_updated"`
	Outputs          []Output         `json:"outputs"`
}

// FlakeInstallable represents the flake reference information.
type FlakeInstallable struct {
	Ref      FlakeRef `json:"ref"`
	AttrPath string   `json:"attr_path"`
}

// FlakeRef represents the git reference for the flake.
type FlakeRef struct {
	Type  string `json:"type"`
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	Rev   string `json:"rev"`
}

// Output represents a package output.
type Output struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Default bool   `json:"default"`
	Nar     string `json:"nar"`
}
