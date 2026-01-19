package fs_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/same/internal/adapters/fs"
	"go.trai.ch/same/internal/core/domain"
)

func TestHasher_ComputeInputHash_ToolsChanges(t *testing.T) {
	tmpDir := t.TempDir()

	// Task with Tool v1
	taskV1 := &domain.Task{
		Name:       domain.NewInternedString("test-task"),
		Tools:      map[string]string{"go": "1.21.0"},
		WorkingDir: domain.NewInternedString("Root"),
	}

	// Task with Tool v2
	taskV2 := &domain.Task{
		Name:       domain.NewInternedString("test-task"),
		Tools:      map[string]string{"go": "1.21.1"},
		WorkingDir: domain.NewInternedString("Root"),
	}

	walker := fs.NewWalker()
	hasher := fs.NewHasher(walker)
	resolver := fs.NewResolver()

	resolvedInputs, err := resolver.ResolveInputs(nil, tmpDir)
	require.NoError(t, err)

	hashV1, err := hasher.ComputeInputHash(taskV1, nil, resolvedInputs)
	require.NoError(t, err)

	hashV2, err := hasher.ComputeInputHash(taskV2, nil, resolvedInputs)
	require.NoError(t, err)

	// Hashes should be different when tools change
	// This assertion is expected to fail before the fix
	assert.NotEqual(t, hashV1, hashV2, "Hash should be different when tools change")
}
