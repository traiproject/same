// Package cas implements Content Addressable Storage and build info storage.
package cas

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"go.trai.ch/bob/internal/core/domain"
	"go.trai.ch/zerr"
)

// Store implements ports.BuildInfoStore using a flat JSON file.
type Store struct {
	path  string
	mu    sync.RWMutex
	cache map[string]domain.BuildInfo
}

// NewStore creates a new BuildInfoStore backed by the file at the given path.
func NewStore(path string) (*Store, error) {
	s := &Store{
		path:  filepath.Clean(path),
		cache: make(map[string]domain.BuildInfo),
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	//nolint:gosec // Path is cleaned and provided by trusted caller
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return zerr.Wrap(err, "failed to read build info store")
	}

	if len(data) == 0 {
		return nil
	}

	if err := json.Unmarshal(data, &s.cache); err != nil {
		return zerr.Wrap(err, "failed to unmarshal build info store")
	}

	return nil
}

func (s *Store) save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(s.cache, "", "  ")
	if err != nil {
		return zerr.Wrap(err, "failed to marshal build info store")
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return zerr.Wrap(err, "failed to create directory for build info store")
	}

	//nolint:gosec // Path is cleaned and provided by trusted caller
	if err := os.WriteFile(s.path, data, 0o644); err != nil {
		return zerr.Wrap(err, "failed to write build info store")
	}

	return nil
}

// Get retrieves the build info for a given task name.
func (s *Store) Get(taskName string) (*domain.BuildInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	info, ok := s.cache[taskName]
	if !ok {
		return nil, nil
	}
	return &info, nil
}

// Put stores the build info.
func (s *Store) Put(info domain.BuildInfo) error {
	// Update cache first
	s.mu.Lock()
	s.cache[info.TaskName] = info
	s.mu.Unlock()

	// Then save to disk
	return s.save()
}
