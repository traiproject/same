package fs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/bob/internal/adapters/fs"
)

func TestResolver_ResolveInputs_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	files := []string{"a.txt", "b.txt", "c.log"}
	for _, f := range files {
		err := os.WriteFile(filepath.Join(tmpDir, f), []byte("content"), 0o600)
		require.NoError(t, err)
	}

	resolver := fs.NewResolver()

	// Test glob pattern
	inputs := []string{"*.txt"}
	resolved, err := resolver.ResolveInputs(inputs, tmpDir)
	require.NoError(t, err)

	// Should match a.txt and b.txt (sorted)
	assert.Len(t, resolved, 2)
	assert.Contains(t, resolved[0], "a.txt")
	assert.Contains(t, resolved[1], "b.txt")
}

func TestResolver_ResolveInputs_GlobError(t *testing.T) {
	tmpDir := t.TempDir()
	resolver := fs.NewResolver()

	// Malformed glob pattern (contains invalid characters)
	inputs := []string{"["}
	_, err := resolver.ResolveInputs(inputs, tmpDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to glob path")
}

func TestResolver_ResolveInputs_NoMatches(t *testing.T) {
	tmpDir := t.TempDir()
	resolver := fs.NewResolver()

	// Pattern that matches nothing
	inputs := []string{"*.nonexistent"}
	_, err := resolver.ResolveInputs(inputs, tmpDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "input not found")
}

func TestResolver_ResolveInputs_MultiplePatterns(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	files := []string{"a.txt", "b.txt", "c.log", "d.log"}
	for _, f := range files {
		err := os.WriteFile(filepath.Join(tmpDir, f), []byte("content"), 0o600)
		require.NoError(t, err)
	}

	resolver := fs.NewResolver()

	// Test multiple patterns
	inputs := []string{"*.txt", "*.log"}
	resolved, err := resolver.ResolveInputs(inputs, tmpDir)
	require.NoError(t, err)

	// Should match all 4 files
	assert.Len(t, resolved, 4)
}

func TestResolver_ResolveInputs_Deduplication(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file
	err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("content"), 0o600)
	require.NoError(t, err)

	resolver := fs.NewResolver()

	// Test with duplicate patterns
	inputs := []string{"test.txt", "*.txt", "test.txt"}
	resolved, err := resolver.ResolveInputs(inputs, tmpDir)
	require.NoError(t, err)

	// Should only have one entry despite duplicates
	assert.Len(t, resolved, 1)
	assert.Contains(t, resolved[0], "test.txt")
}

func TestResolver_ResolveInputs_Sorting(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files in non-alphabetical order
	files := []string{"z.txt", "a.txt", "m.txt"}
	for _, f := range files {
		err := os.WriteFile(filepath.Join(tmpDir, f), []byte("content"), 0o600)
		require.NoError(t, err)
	}

	resolver := fs.NewResolver()

	// Resolve all files
	inputs := []string{"*.txt"}
	resolved, err := resolver.ResolveInputs(inputs, tmpDir)
	require.NoError(t, err)

	// Should be sorted alphabetically
	assert.Len(t, resolved, 3)
	assert.Contains(t, resolved[0], "a.txt")
	assert.Contains(t, resolved[1], "m.txt")
	assert.Contains(t, resolved[2], "z.txt")
}
