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

	"go.trai.ch/same/internal/core/domain"
	"go.trai.ch/zerr"
)

// Store implements ports.BuildInfoStore using a file-per-task strategy.
type Store struct{}

// NewStore creates a new BuildInfoStore backed by the directory at the given path.
func NewStore() (*Store, error) {
	return &Store{}, nil
}

// newStoreWithPath is retained for test compatibility but no longer uses the path parameter.
// All operations now require an explicit root parameter. Tests should pass tmpDir as root to Get/Put.
func newStoreWithPath(_ string) (*Store, error) {
	return &Store{}, nil
}

// Get retrieves the build info for a given task name.
func (s *Store) Get(root, taskName string) (*domain.BuildInfo, error) {
	filename := s.getFilename(root, taskName)
	//nolint:gosec // Path is constructed from trusted directory and hashed filename
	data, err := os.ReadFile(filename)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, zerr.Wrap(err, domain.ErrStoreReadFailed.Error())
	}

	var info domain.BuildInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, zerr.Wrap(err, domain.ErrStoreUnmarshalFailed.Error())
	}

	return &info, nil
}

// Put stores the build info.
func (s *Store) Put(root string, info domain.BuildInfo) error {
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return zerr.Wrap(err, domain.ErrStoreMarshalFailed.Error())
	}

	filename := s.getFilename(root, info.TaskName)
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, domain.DirPerm); err != nil {
		return zerr.Wrap(err, domain.ErrStoreCreateFailed.Error())
	}

	//nolint:gosec // Path is constructed from trusted directory and hashed filename
	if err := os.WriteFile(filename, data, domain.FilePerm); err != nil {
		return zerr.Wrap(err, domain.ErrStoreWriteFailed.Error())
	}

	return nil
}

func (s *Store) getFilename(root, taskName string) string {
	hash := sha256.Sum256([]byte(taskName))
	hexHash := hex.EncodeToString(hash[:])
	storeDir := filepath.Join(root, domain.DefaultStorePath())
	return filepath.Join(storeDir, hexHash+".json")
}
