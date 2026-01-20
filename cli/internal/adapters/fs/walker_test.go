package fs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/same/internal/adapters/fs"
)

func TestWalker_WalkFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test structure
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "dir1"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "dir2"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content1"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "dir1", "file2.txt"), []byte("content2"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "dir2", "file3.txt"), []byte("content3"), 0o600))

	walker := fs.NewWalker()
	files := make([]string, 0)

	for filePath := range walker.WalkFiles(tmpDir, nil) {
		files = append(files, filePath)
	}

	// Should find 3 files
	assert.Len(t, files, 3)
	assert.Contains(t, files, filepath.Join(tmpDir, "file1.txt"))
	assert.Contains(t, files, filepath.Join(tmpDir, "dir1", "file2.txt"))
	assert.Contains(t, files, filepath.Join(tmpDir, "dir2", "file3.txt"))
}

func TestWalker_WalkFiles_SkipsGitAndJJ(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test structure with .git and .jj directories
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".git", "objects"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".jj"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "src"), 0o750))

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".git", "config"), []byte("gitconfig"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".jj", "store"), []byte("jjstore"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "src", "main.go"), []byte("package main"), 0o600))

	walker := fs.NewWalker()
	files := make([]string, 0)

	for filePath := range walker.WalkFiles(tmpDir, nil) {
		files = append(files, filePath)
	}

	// Should only find src/main.go, not .git or .jj files
	assert.Len(t, files, 1)
	assert.Contains(t, files, filepath.Join(tmpDir, "src", "main.go"))
}

func TestWalker_WalkFiles_WithIgnores(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test structure
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "build"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main_test.go"), []byte("package main"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "build", "output.bin"), []byte("binary"), 0o600))

	walker := fs.NewWalker()
	files := make([]string, 0)

	// Ignore *_test.go files and build directory
	ignores := []string{"*_test.go", "build"}
	for filePath := range walker.WalkFiles(tmpDir, ignores) {
		files = append(files, filePath)
	}

	// shouldSkipDir skips directories but not files, so both files will be found
	// The build directory is skipped, so build/output.bin won't appear
	assert.Len(t, files, 2)
	assert.Contains(t, files, filepath.Join(tmpDir, "main.go"))
	assert.Contains(t, files, filepath.Join(tmpDir, "main_test.go"))
}

func TestWalker_WalkFiles_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	walker := fs.NewWalker()
	files := make([]string, 0)

	for filePath := range walker.WalkFiles(tmpDir, nil) {
		files = append(files, filePath)
	}

	// Should find no files
	assert.Empty(t, files)
}
