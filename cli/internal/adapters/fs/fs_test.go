package fs_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/same/internal/adapters/fs"
	"go.trai.ch/same/internal/core/domain"
)

func TestHasher_ComputeInputHash(t *testing.T) {
	// Helper to create a dummy task
	createTask := func() *domain.Task {
		return &domain.Task{
			Name:    domain.NewInternedString("test-task"),
			Command: []string{"echo", "hello"},
			Tools:   map[string]string{"go": "1.21"},
		}
	}

	t.Run("Content Change", func(t *testing.T) {
		tmpDir := t.TempDir()
		file := filepath.Join(tmpDir, "file.txt")

		// Create initial file
		require.NoError(t, os.WriteFile(file, []byte("content1"), domain.PrivateFilePerm))

		walker := fs.NewWalker()
		hasher := fs.NewHasher(walker)
		task := createTask()
		env := map[string]string{"FOO": "bar"}

		hash1, err := hasher.ComputeInputHash(task, env, []string{file})
		require.NoError(t, err)

		// Change content
		require.NoError(t, os.WriteFile(file, []byte("content2"), domain.PrivateFilePerm))

		hash2, err := hasher.ComputeInputHash(task, env, []string{file})
		require.NoError(t, err)

		assert.NotEqual(t, hash1, hash2, "Hash should change when content changes")
	})

	t.Run("Metadata Change", func(t *testing.T) {
		tmpDir := t.TempDir()
		file := filepath.Join(tmpDir, "file.txt")

		// Create initial file
		require.NoError(t, os.WriteFile(file, []byte("content"), domain.PrivateFilePerm))

		walker := fs.NewWalker()
		hasher := fs.NewHasher(walker)
		task := createTask()
		env := map[string]string{"FOO": "bar"}

		hash1, err := hasher.ComputeInputHash(task, env, []string{file})
		require.NoError(t, err)

		// Touch file (change mtime only)
		futureTime := time.Now().Add(1 * time.Hour)
		require.NoError(t, os.Chtimes(file, futureTime, futureTime))

		hash2, err := hasher.ComputeInputHash(task, env, []string{file})
		require.NoError(t, err)

		assert.Equal(t, hash1, hash2, "Hash should NOT change when only metadata (mtime) changes")
	})

	t.Run("Ordering", func(t *testing.T) {
		tmpDir := t.TempDir()
		file1 := filepath.Join(tmpDir, "a.txt")
		file2 := filepath.Join(tmpDir, "b.txt")

		require.NoError(t, os.WriteFile(file1, []byte("A"), domain.PrivateFilePerm))
		require.NoError(t, os.WriteFile(file2, []byte("B"), domain.PrivateFilePerm))

		walker := fs.NewWalker()
		hasher := fs.NewHasher(walker)
		task := createTask()
		env := map[string]string{"FOO": "bar"}

		// Order 1: [a.txt, b.txt]
		hash1, err := hasher.ComputeInputHash(task, env, []string{file1, file2})
		require.NoError(t, err)

		// Order 2: [b.txt, a.txt]
		// Order 2: [b.txt, a.txt]
		// Verify that hashing is order-independent (inputs should be sorted by hasher or caller).
		// Note: Current implementation might rely on caller sorting, this test enforces sorting expectation.

		hash2, err := hasher.ComputeInputHash(task, env, []string{file2, file1})
		require.NoError(t, err)

		assert.Equal(t, hash1, hash2, "Hash should be independent of input file order")
	})
}
