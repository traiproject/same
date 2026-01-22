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

func TestHasher_ComputeOutputHash_Deterministic(t *testing.T) {
	// Create a temp directory
	tmpDir := t.TempDir()

	// Create some nested files
	// We want to verify that regardless of the input order of outputs
	// the hash is the same.
	// AND if we pass a directory, we want to ensure the verification of contents is deterministic.
	// (Note: Implementation of walker needs to be deterministic for directory hashing to be stable if walker is used)

	err := os.MkdirAll(filepath.Join(tmpDir, "subdir"), domain.DirPerm)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content1"), domain.PrivateFilePerm)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "subdir", "file2.txt"), []byte("content2"), domain.PrivateFilePerm)
	require.NoError(t, err)

	walker := fs.NewWalker()
	hasher := fs.NewHasher(walker)

	// Case 1: List of files passed as outputs, order shouldn't matter
	outputs1 := []string{"file1.txt", "subdir/file2.txt"}
	outputs2 := []string{"subdir/file2.txt", "file1.txt"}

	hash1, err := hasher.ComputeOutputHash(outputs1, tmpDir)
	require.NoError(t, err)

	hash2, err := hasher.ComputeOutputHash(outputs2, tmpDir)
	require.NoError(t, err)

	assert.Equal(t, hash1, hash2, "Hash should be deterministic regardless of output list order")
}

func TestHasher_ComputeOutputHash_StatError(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "unreadable.txt")

	err := os.WriteFile(filePath, []byte("secret"), 0o000) // No permissions
	require.NoError(t, err)

	// Ensure we restore permissions so cleanup works
	defer func() {
		_ = os.Chmod(filePath, domain.PrivateFilePerm)
	}()

	walker := fs.NewWalker()
	hasher := fs.NewHasher(walker)

	_, err = hasher.ComputeOutputHash([]string{"unreadable.txt"}, tmpDir)
	require.Error(t, err)
	// We check for the error string or wrapped error
	// Implementation wraps with zerr.Wrap(err, domain.ErrPathStatFailed) OR domain.ErrFileOpenFailed

	// So we expect read failure
	assert.Contains(t, err.Error(), "failed to open file")
}

func TestResolver_ResolveInputs_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	resolver := fs.NewResolver()

	// Matches nothing
	_, err := resolver.ResolveInputs([]string{"nonexistent*.txt"}, tmpDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "input not found")
}

func TestResolver_ResolveInputs_MalformedGlob(t *testing.T) {
	tmpDir := t.TempDir()

	resolver := fs.NewResolver()

	// Malformed glob
	_, err := resolver.ResolveInputs([]string{"["}, tmpDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "syntax error")
}
