package fs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.trai.ch/same/internal/adapters/fs"
	"go.trai.ch/same/internal/core/domain"
)

// expectedHash is the hardcoded golden hash for the synthetic task.
// If this changes, it means we have broken cache compatibility for all users.
// Validate the change carefully before updating this constant.
const expectedHash = "0202c01740364154"

func TestHasher_ComputeInputHash_Golden(t *testing.T) {
	// 1. Setup a dummy file structure
	tmpDir := t.TempDir()
	dummyFile := filepath.Join(tmpDir, "dummy.txt")
	err := os.WriteFile(dummyFile, []byte("start-content"), domain.PrivateFilePerm)
	require.NoError(t, err)

	// 2. Create a synthetic task with FIXED values
	task := &domain.Task{
		Name:         domain.NewInternedString("build-web"),
		Command:      []string{"go", "build", "./..."},
		Inputs:       []domain.InternedString{domain.NewInternedString("dummy.txt")},
		Outputs:      []domain.InternedString{domain.NewInternedString("bin/web")},
		Tools:        map[string]string{"go": "1.25.4"},
		Dependencies: []domain.InternedString{domain.NewInternedString("lint")},
		Environment:  map[string]string{"CGO_ENABLED": "0"},
		WorkingDir:   domain.NewInternedString("."),
	}

	env := map[string]string{
		"HOME": "/users/test",
		"TERM": "xterm-256color",
	}

	// 3. Initialize Hasher
	// We need to change directory to tmpDir so the relative path "dummy.txt" works
	// or we can pass absolute path. domain.Task inputs are usually relative.
	// Hasher uses walker.WalkFiles(path, nil) or hashFile(path).
	// Let's change Cwd for the test or use absolute paths for the inputs?
	// Hasher logic: hashPath calls os.Stat(path).
	// If the Input is "dummy.txt", it looks for it in Cwd.
	// For this test, let's switch Cwd to tmpDir.
	wd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(wd) }()
	require.NoError(t, os.Chdir(tmpDir))

	walker := fs.NewWalker()
	hasher := fs.NewHasher(walker)

	// 4. Compute Hash
	// Inputs must be passed as resolved paths if they are not relative to Cwd.
	// Here they are relative to Cwd (tmpDir).
	inputs := []string{"dummy.txt"}

	hash, err := hasher.ComputeInputHash(task, env, inputs)
	require.NoError(t, err)

	// 5. Assert against Golden Hash
	require.Equal(t, expectedHash, hash, "Hasher algorithm changed! Verify if this is intentional.")
}
