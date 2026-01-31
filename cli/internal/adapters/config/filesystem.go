package config

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// FileSystem abstracts filesystem operations for testability.
type FileSystem interface {
	// Stat returns file info for the given path.
	Stat(path string) (fs.FileInfo, error)
	// ReadFile reads the entire file at path.
	ReadFile(path string) ([]byte, error)
	// Glob returns matches for the given pattern.
	Glob(pattern string) ([]string, error)
	// IsDir checks if the path is a directory.
	IsDir(path string) (bool, error)
}

// OSFS implements FileSystem using the standard library.
type OSFS struct{}

// NewOSFS creates a new OSFS instance.
func NewOSFS() *OSFS {
	return &OSFS{}
}

// Stat returns file info for the given path.
func (o *OSFS) Stat(path string) (fs.FileInfo, error) {
	return os.Stat(path)
}

// ReadFile reads the entire file at path.
func (o *OSFS) ReadFile(path string) ([]byte, error) {
	// #nosec G304 -- path is validated by caller
	return os.ReadFile(path)
}

// Glob returns matches for the given pattern.
func (o *OSFS) Glob(pattern string) ([]string, error) {
	return filepath.Glob(pattern)
}

// IsDir checks if the path is a directory.
func (o *OSFS) IsDir(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}

// MapFSAdapter adapts fstest.MapFS to FileSystem interface for testing.
type MapFSAdapter struct {
	FS   fs.FS
	Root string // simulated root path
}

// NewMapFSAdapter creates a new MapFSAdapter with the given root path and filesystem.
func NewMapFSAdapter(root string, fsys fs.FS) *MapFSAdapter {
	return &MapFSAdapter{
		FS:   fsys,
		Root: root,
	}
}

// Stat returns file info for the given path.
func (m *MapFSAdapter) Stat(path string) (fs.FileInfo, error) {
	relPath := m.toRelPath(path)
	return fs.Stat(m.FS, relPath)
}

// ReadFile reads the entire file at path.
func (m *MapFSAdapter) ReadFile(path string) ([]byte, error) {
	relPath := m.toRelPath(path)
	return fs.ReadFile(m.FS, relPath)
}

// Glob returns matches for the given pattern.
// For MapFS, we match against all files in the filesystem.
// NOTE: Unlike filepath.Glob, this implementation only returns directories,
// not regular files. This is intentional for the workspace project discovery use case.
func (m *MapFSAdapter) Glob(pattern string) ([]string, error) {
	// Convert pattern to relative form
	relPattern := m.toRelPath(pattern)

	// Collect all matching paths
	var matches []string
	err := fs.WalkDir(m.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Check if this path matches the glob pattern
		matched, err := filepath.Match(relPattern, path)
		if err != nil {
			return err
		}
		if matched && d.IsDir() {
			// Return absolute path
			matches = append(matches, filepath.Join(m.Root, path))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return matches, nil
}

// IsDir checks if the path is a directory.
func (m *MapFSAdapter) IsDir(path string) (bool, error) {
	info, err := m.Stat(path)
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}

// toRelPath converts an absolute path to a relative path within the filesystem.
// If the path is outside the root, it returns the path unchanged, which will cause
// downstream fs operations to fail with "file not found" errors.
func (m *MapFSAdapter) toRelPath(absPath string) string {
	// Handle the case where absPath is already relative
	if !filepath.IsAbs(absPath) {
		return absPath
	}

	// Validate that absPath is within m.Root (with proper path boundary check)
	// Special case: if root is "/", all absolute paths are within root
	if m.Root != "/" && absPath != m.Root && !strings.HasPrefix(absPath, m.Root+string(filepath.Separator)) {
		// Path is outside root - return as-is and let fs operations fail with clear error
		return absPath
	}

	// Strip the root prefix
	rel := strings.TrimPrefix(absPath, m.Root)
	rel = strings.TrimPrefix(rel, string(filepath.Separator))
	return rel
}
