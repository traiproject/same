// Package cas implements Content Addressable Storage and build info storage.
package cas

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"go.trai.ch/bob/internal/core/domain"
	"go.trai.ch/zerr"
)

const (
	dirPerm  = 0o750
	filePerm = 0o644
)

// Store implements ports.BuildInfoStore using a file-per-task strategy.
type Store struct {
	dir string
}

// NewStore creates a new BuildInfoStore backed by the directory at the given path.
func NewStore(path string) (*Store, error) {
	cleanPath := filepath.Clean(path)
	if err := os.MkdirAll(cleanPath, dirPerm); err != nil {
		return nil, zerr.Wrap(err, "failed to create build info store directory")
	}

	return &Store{
		dir: cleanPath,
	}, nil
}

// Get retrieves the build info for a given task name.
func (s *Store) Get(taskName string) (*domain.BuildInfo, error) {
	filename := s.getFilename(taskName)
	//nolint:gosec // Path is constructed from trusted directory and hashed filename
	data, err := os.ReadFile(filename)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, zerr.Wrap(err, "failed to read build info")
	}

	var info domain.BuildInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, zerr.Wrap(err, "failed to unmarshal build info")
	}

	return &info, nil
}

// Put stores the build info.
func (s *Store) Put(info domain.BuildInfo) error {
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return zerr.Wrap(err, "failed to marshal build info")
	}

	filename := s.getFilename(info.TaskName)
	//nolint:gosec // Path is constructed from trusted directory and hashed filename
	if err := os.WriteFile(filename, data, filePerm); err != nil {
		return zerr.Wrap(err, "failed to write build info")
	}

	return nil
}

func (s *Store) getFilename(taskName string) string {
	hash := sha256.Sum256([]byte(taskName))
	hexHash := hex.EncodeToString(hash[:])
	return filepath.Join(s.dir, hexHash+".json")
}
