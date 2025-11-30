package nix //nolint:testpackage // Allow testing internals

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateNixExpression(t *testing.T) {
	tests := []struct {
		name     string
		deps     []string
		expected string
	}{
		{
			name:     "empty",
			deps:     []string{},
			expected: "let pkgs = import <nixpkgs> {}; in pkgs.mkShell { buildInputs = []; }",
		},
		{
			name:     "single dependency",
			deps:     []string{"git"},
			expected: "let pkgs = import <nixpkgs> {}; in pkgs.mkShell { buildInputs = with pkgs; [ git ]; }",
		},
		{
			name:     "multiple dependencies",
			deps:     []string{"git", "go"},
			expected: "let pkgs = import <nixpkgs> {}; in pkgs.mkShell { buildInputs = with pkgs; [ git go ]; }",
		},
		{
			name:     "dependencies with quotes",
			deps:     []string{"foo\"bar"},
			expected: "let pkgs = import <nixpkgs> {}; in pkgs.mkShell { buildInputs = with pkgs; [ foo\"bar ]; }",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateNixExpression(tt.deps)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestComputeHash(t *testing.T) {
	deps1 := []string{"a", "b"}
	deps2 := []string{"a", "b"}
	deps3 := []string{"b", "a"} // Order matters for input to computeHash, but Resolve sorts them first

	h1, err := computeHash(deps1)
	require.NoError(t, err)

	h2, err := computeHash(deps2)
	require.NoError(t, err)

	h3, err := computeHash(deps3)
	require.NoError(t, err)

	assert.Equal(t, h1, h2)
	assert.NotEqual(t, h1, h3)
}

func TestResolve_CacheHit(t *testing.T) {
	// Create a temporary cache file
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "nix_cache.json")

	// Pre-populate cache
	deps := []string{"git"}
	hash, err := computeHash(deps) // "git" is already sorted
	require.NoError(t, err)

	expectedVars := map[string]string{
		"PATH": "/nix/store/something/bin",
	}
	cacheData := map[string]map[string]string{
		hash: expectedVars,
	}
	data, err := json.Marshal(cacheData)
	require.NoError(t, err)
	err = os.WriteFile(cachePath, data, 0o600)
	require.NoError(t, err)

	// Create adapter
	adapter := New(cachePath)

	// Resolve
	vars, err := adapter.Resolve(context.Background(), deps)
	require.NoError(t, err)
	assert.Equal(t, expectedVars, vars)
}
