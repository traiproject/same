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

const testHashValue = "testhash"

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
	// NOTE: The error returns in computeHash are part of the io.Writer interface contract
	// but are unreachable in practice. sha256.Hash.Write() for in-memory operations with
	// valid strings never fails. We document this limitation and focus on correctness tests.

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

// --- updateCache Tests ---.

func TestUpdateCache_NewCache(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "subdir", "nix_cache.json")
	adapter := New(cachePath)

	hash := "testhash123"
	vars := map[string]string{
		"PATH": "/nix/store/foo/bin",
		"HOME": "/home/user",
	}

	err := adapter.updateCache(hash, vars)
	require.NoError(t, err)

	// Verify directory was created
	_, err = os.Stat(filepath.Dir(cachePath))
	require.NoError(t, err)

	// Verify cache file was written with correct structure
	content, err := os.ReadFile(cachePath) //nolint:gosec // Test file, path is controlled
	require.NoError(t, err)

	var cache cacheFile
	err = json.Unmarshal(content, &cache)
	require.NoError(t, err)

	assert.Equal(t, vars, cache[hash])
}

func TestUpdateCache_ExistingCache(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "nix_cache.json")

	// Pre-populate with existing entry
	existingHash := "existing123"
	existingVars := map[string]string{"OLD": "value"}
	existingCache := cacheFile{
		existingHash: existingVars,
	}
	data, err := json.Marshal(existingCache)
	require.NoError(t, err)
	err = os.WriteFile(cachePath, data, 0o600)
	require.NoError(t, err)

	adapter := New(cachePath)

	// Add new entry
	newHash := "newhash456"
	newVars := map[string]string{
		"PATH": "/nix/store/bar/bin",
	}

	err = adapter.updateCache(newHash, newVars)
	require.NoError(t, err)

	// Verify both entries exist
	content, err := os.ReadFile(cachePath) //nolint:gosec // Test file, path is controlled
	require.NoError(t, err)

	var cache cacheFile
	err = json.Unmarshal(content, &cache)
	require.NoError(t, err)

	assert.Equal(t, existingVars, cache[existingHash])
	assert.Equal(t, newVars, cache[newHash])

	// Verify JSON is indented (MarshalIndent was used)
	assert.Contains(t, string(content), "\n  ")
}

func TestUpdateCache_CorruptedCache(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "nix_cache.json")

	// Create corrupted cache
	err := os.WriteFile(cachePath, []byte("this is not valid JSON{{{"), 0o600)
	require.NoError(t, err)

	adapter := New(cachePath)

	// Add new entry - should overwrite corrupted cache
	newHash := "newhash789"
	newVars := map[string]string{
		"PATH": "/nix/store/baz/bin",
	}

	err = adapter.updateCache(newHash, newVars)
	require.NoError(t, err)

	// Verify cache contains only the new entry
	content, err := os.ReadFile(cachePath) //nolint:gosec // Test file, path is controlled
	require.NoError(t, err)

	var cache cacheFile
	err = json.Unmarshal(content, &cache)
	require.NoError(t, err)

	assert.Len(t, cache, 1)
	assert.Equal(t, newVars, cache[newHash])
}

func TestUpdateCache_DirectoryCreationError(t *testing.T) {
	// Use a path that cannot have directories created under it
	// On Unix systems, /dev/null is a device file, not a directory
	cachePath := "/dev/null/impossible/nix_cache.json"
	adapter := New(cachePath)

	hash := testHashValue
	vars := map[string]string{"PATH": "/test"}

	err := adapter.updateCache(hash, vars)
	assert.Error(t, err)
}

func TestUpdateCache_WriteFileError(t *testing.T) {
	// Create a directory where the cache file should be
	// This will cause os.WriteFile to fail
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "nix_cache.json")

	// Create a directory with the same name as the cache file
	err := os.Mkdir(cachePath, 0o750)
	require.NoError(t, err)

	adapter := New(cachePath)

	hash := testHashValue
	vars := map[string]string{"PATH": "/test"}

	// Should fail because cachePath is a directory, not a file
	err = adapter.updateCache(hash, vars)
	assert.Error(t, err)
}

// --- checkCache Tests ---.

func TestCheckCache_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "nonexistent_cache.json")
	adapter := New(cachePath)

	vars, ok := adapter.checkCache("anyhash")
	assert.Nil(t, vars)
	assert.False(t, ok)
}

func TestCheckCache_CorruptedJSON(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "nix_cache.json")

	// Create corrupted JSON
	err := os.WriteFile(cachePath, []byte("not-valid-json"), 0o600)
	require.NoError(t, err)

	adapter := New(cachePath)

	vars, ok := adapter.checkCache("anyhash")
	assert.Nil(t, vars)
	assert.False(t, ok)
}

func TestCheckCache_EmptyCache(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "nix_cache.json")

	// Create valid but empty cache
	err := os.WriteFile(cachePath, []byte("{}"), 0o600)
	require.NoError(t, err)

	adapter := New(cachePath)

	vars, ok := adapter.checkCache("nonexistenthash")
	assert.Nil(t, vars)
	assert.False(t, ok)
}

// --- Resolve Error Path Tests ---.

func TestResolve_UpdateCacheError(t *testing.T) {
	// Skip this test if not running as root, as we need permission constraints
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "readonly", "nix_cache.json")

	// Create the directory
	readonlyDir := filepath.Dir(cachePath)
	err := os.MkdirAll(readonlyDir, 0o750)
	require.NoError(t, err)

	// Make directory read-only (on Unix systems)
	err = os.Chmod(readonlyDir, 0o444) //nolint:gosec // Need to test error handling
	require.NoError(t, err)
	defer func() {
		// Restore permissions for cleanup
		_ = os.Chmod(readonlyDir, 0o750) //nolint:gosec // Cleanup step
	}()

	adapter := New(cachePath)

	// This will fail because we can't execute nix command with invalid deps
	// but if we could, the updateCache would fail due to read-only dir
	// For now, we test updateCache directly
	hash := testHashValue
	vars := map[string]string{"PATH": "/test"}

	err = adapter.updateCache(hash, vars)
	assert.Error(t, err)
}

// --- computeHash Enhanced Tests ---.

func TestComputeHash_EmptySlice(t *testing.T) {
	h1, err := computeHash([]string{})
	require.NoError(t, err)

	h2, err := computeHash([]string{})
	require.NoError(t, err)

	// Empty slice should produce consistent hash
	assert.Equal(t, h1, h2)
	assert.NotEmpty(t, h1)
}

func TestComputeHash_CollisionResistance(t *testing.T) {
	// Test that separator prevents collisions
	deps1 := []string{"ab", "c"}
	deps2 := []string{"a", "bc"}

	h1, err := computeHash(deps1)
	require.NoError(t, err)

	h2, err := computeHash(deps2)
	require.NoError(t, err)

	// These should produce different hashes due to null byte separator
	assert.NotEqual(t, h1, h2)
}

// --- Resolve Dependency Sorting Tests ---.

func TestResolve_DependencySorting(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "nix_cache.json")
	adapter := New(cachePath)

	// Dependencies provided in different order but should result in cache hit
	deps1 := []string{"git", "go", "curl"}
	deps2 := []string{"curl", "git", "go"}

	// Pre-populate cache with sorted version
	sortedDeps := []string{"curl", "git", "go"}
	hash, err := computeHash(sortedDeps)
	require.NoError(t, err)

	expectedVars := map[string]string{
		"PATH": "/nix/store/sorted/bin",
	}
	cacheData := map[string]map[string]string{
		hash: expectedVars,
	}
	data, err := json.Marshal(cacheData)
	require.NoError(t, err)
	err = os.WriteFile(cachePath, data, 0o600)
	require.NoError(t, err)

	// Both orders should hit cache
	vars1, err := adapter.Resolve(context.Background(), deps1)
	require.NoError(t, err)
	assert.Equal(t, expectedVars, vars1)

	vars2, err := adapter.Resolve(context.Background(), deps2)
	require.NoError(t, err)
	assert.Equal(t, expectedVars, vars2)
}

func TestResolve_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "nix_cache.json")
	adapter := New(cachePath)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// This should fail quickly due to canceled context
	// The error will come from exec.CommandContext when it tries to run
	_, err := adapter.Resolve(ctx, []string{"nonexistent-package-xyz"})

	// We expect an error, though the exact error depends on timing
	// It could be context.Canceled or a nix command failure
	assert.Error(t, err)
}
