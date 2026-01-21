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
	t.Parallel()

	// Create a temporary directory structure
	rootDir := t.TempDir()

	// structure:
	// /root
	//   /file1.txt
	//   /subdir
	//     /file2.go
	//   /.git
	//     /config
	//   /.jj
	//     /repo
	//   /ignored_dir
	//     /ignored.txt

	mustCreateFile(t, rootDir, "file1.txt")
	mustCreateDir(t, rootDir, "subdir")
	mustCreateFile(t, rootDir, "subdir/file2.go")

	mustCreateDir(t, rootDir, ".git")
	mustCreateFile(t, rootDir, ".git/config")

	mustCreateDir(t, rootDir, ".jj")
	mustCreateFile(t, rootDir, ".jj/repo")

	mustCreateDir(t, rootDir, "ignored_dir")
	mustCreateFile(t, rootDir, "ignored_dir/ignored.txt")

	walker := fs.NewWalker()

	t.Run("defaults (skip .git and .jj)", func(t *testing.T) {
		t.Parallel()
		var visited []string
		for path := range walker.WalkFiles(rootDir, nil) {
			rel, err := filepath.Rel(rootDir, path)
			require.NoError(t, err)
			visited = append(visited, rel)
		}

		assert.Contains(t, visited, "file1.txt")
		assert.Contains(t, visited, "subdir/file2.go")
		assert.Contains(t, visited, "ignored_dir/ignored.txt") // Not ignored yet
		assert.NotContains(t, visited, ".git/config")
		assert.NotContains(t, visited, ".jj/repo")
	})

	t.Run("with ignores", func(t *testing.T) {
		t.Parallel()
		var visited []string
		for path := range walker.WalkFiles(rootDir, []string{"ignored_dir"}) {
			rel, err := filepath.Rel(rootDir, path)
			require.NoError(t, err)
			visited = append(visited, rel)
		}

		assert.Contains(t, visited, "file1.txt")
		assert.Contains(t, visited, "subdir/file2.go")
		assert.NotContains(t, visited, "ignored_dir/ignored.txt")
		assert.NotContains(t, visited, ".git/config")
		assert.NotContains(t, visited, ".jj/repo")
	})
}

func mustCreateDir(t *testing.T, root, name string) {
	t.Helper()
	err := os.MkdirAll(filepath.Join(root, name), 0o750)
	require.NoError(t, err)
}

func mustCreateFile(t *testing.T, root, name string) {
	t.Helper()
	path := filepath.Join(root, name)
	//nolint:gosec // 0600 is fine for test
	err := os.WriteFile(path, []byte("content"), 0o600)
	require.NoError(t, err)
}
