// Package fs provides file system adapters for walking and hashing files.
package fs

import (
	"io/fs"
	"iter"
	"path/filepath"
)

// Walker provides file walking functionality.
type Walker struct{}

// NewWalker creates a new Walker.
func NewWalker() *Walker {
	return &Walker{}
}

// WalkFiles yields all files in the root directory, skipping .git and ignored directories.
// It returns an iterator that yields file paths relative to the root (or absolute if root is absolute,
// but typically WalkDir yields paths including root).
// Actually, filepath.WalkDir yields paths starting with root.
func (w *Walker) WalkFiles(root string, ignores []string) iter.Seq[string] {
	return func(yield func(string) bool) {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// Check if we should skip this directory
			if skipAction := w.shouldSkipDir(d, ignores); skipAction != nil {
				return skipAction
			}

			// Skip directories, yield files
			if d.IsDir() {
				return nil
			}

			if !yield(path) {
				return filepath.SkipAll
			}

			return nil
		})
	}
}

// shouldSkipDir checks if a directory should be skipped based on ignore patterns.
// Returns filepath.SkipDir if the directory should be skipped, nil if file should be skipped, or nil to continue.
func (w *Walker) shouldSkipDir(d fs.DirEntry, ignores []string) error {
	name := d.Name()

	// Always skip .git
	if d.IsDir() && name == ".git" {
		return filepath.SkipDir
	}

	// Always skip .jj
	if d.IsDir() && name == ".jj" {
		return filepath.SkipDir
	}

	// Check ignores
	for _, ignore := range ignores {
		matched, _ := filepath.Match(ignore, name)
		if matched {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil // Skip file
		}
	}

	return nil
}
