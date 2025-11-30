// Package nix implements the Environment interface using Nix.
package nix

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"go.trai.ch/bob/internal/core/ports"
	"go.trai.ch/zerr"
)

const (
	dirPerm  = 0o750
	filePerm = 0o600
)

// Adapter implements ports.Environment using Nix.
type Adapter struct {
	cachePath string
}

// New creates a new Nix adapter.
// cachePath is the path to the JSON file used for caching resolved environments.
func New(cachePath string) *Adapter {
	return &Adapter{
		cachePath: cachePath,
	}
}

// Resolve resolves the environment variables for the given system dependencies.
func (a *Adapter) Resolve(ctx context.Context, dependencies []string) (map[string]string, error) {
	// 1. Sort dependencies for deterministic hashing
	sort.Strings(dependencies)

	// 2. Compute hash
	hash, err := computeHash(dependencies)
	if err != nil {
		return nil, zerr.Wrap(err, "failed to compute dependency hash")
	}

	// 3. Check cache
	if cached, ok := a.checkCache(hash); ok {
		return cached, nil
	}

	// 4. Generate Nix expression
	expr := generateNixExpression(dependencies)

	// 5. Execute Nix
	// nix print-dev-env --json --expr '...'
	//nolint:gosec // expr is generated from sanitized dependencies
	cmd := exec.CommandContext(ctx, "nix", "print-dev-env",
		"--extra-experimental-features", "nix-command flakes",
		"--json", "--expr", expr)
	output, err := cmd.Output()
	if err != nil {
		// Try to capture stderr for better error reporting
		var stderr string
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = string(exitErr.Stderr)
		}
		return nil, zerr.With(zerr.With(zerr.Wrap(err, "nix command failed"),
			"stderr", stderr),
			"expression", expr,
		)
	}

	// 6. Parse output
	var envData struct {
		Variables map[string]struct {
			Type  string `json:"type"`
			Value string `json:"value"`
		} `json:"variables"`
	}
	if err := json.Unmarshal(output, &envData); err != nil {
		return nil, zerr.With(zerr.Wrap(err, "failed to parse nix output"), "output", string(output))
	}

	// 7. Extract variables
	vars := make(map[string]string)
	for k, v := range envData.Variables {
		// We only care about exported variables, usually type "exported" or "var"
		// The output of print-dev-env --json has "type": "exported" for env vars.
		if v.Type == "exported" {
			vars[k] = v.Value
		}
	}

	// 8. Update cache
	if err := a.updateCache(hash, vars); err != nil {
		// Log error but don't fail the operation?
		// For now, let's just return the error as it might indicate fs issues
		return nil, zerr.Wrap(err, "failed to update cache")
	}

	return vars, nil
}

func computeHash(deps []string) (string, error) {
	h := sha256.New()
	for _, dep := range deps {
		if _, err := h.Write([]byte(dep)); err != nil {
			return "", err
		}
		// Add a separator to avoid collisions like ["ab", "c"] vs ["a", "bc"]
		if _, err := h.Write([]byte{0}); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

type cacheFile map[string]map[string]string

func (a *Adapter) checkCache(hash string) (map[string]string, bool) {
	f, err := os.Open(a.cachePath)
	if err != nil {
		return nil, false
	}
	defer func() { _ = f.Close() }()

	var cache cacheFile
	if err := json.NewDecoder(f).Decode(&cache); err != nil {
		return nil, false
	}

	val, ok := cache[hash]
	return val, ok
}

func (a *Adapter) updateCache(hash string, vars map[string]string) error {
	// Read existing cache
	cache := make(cacheFile)
	content, err := os.ReadFile(a.cachePath)
	if err == nil {
		if jsonErr := json.Unmarshal(content, &cache); jsonErr != nil {
			// If cache is corrupted, ignore and overwrite
			_ = jsonErr
		}
	}

	// Update
	cache[hash] = vars

	// Write back
	// Ensure directory exists
	dir := filepath.Dir(a.cachePath)
	if mkErr := os.MkdirAll(dir, dirPerm); mkErr != nil {
		return mkErr
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(a.cachePath, data, filePerm)
}

// Ensure Adapter satisfies the interface.
var _ ports.Environment = (*Adapter)(nil)
