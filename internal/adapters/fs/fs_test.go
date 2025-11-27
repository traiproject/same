package fs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/bob/internal/adapters/fs"
	"go.trai.ch/bob/internal/core/domain"
)

func TestHasher_ComputeInputHash_Glob(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create files
	files := []string{"a.txt", "b.txt", "c.log"}
	for _, f := range files {
		err := os.WriteFile(filepath.Join(tmpDir, f), []byte("content"), 0o600)
		require.NoError(t, err)
	}

	// Define task with glob input
	task := &domain.Task{
		Name:   domain.NewInternedString("test-task"),
		Inputs: []domain.InternedString{domain.NewInternedString("*.txt")},
	}

	// Initialize Hasher
	walker := fs.NewWalker()
	hasher := fs.NewHasher(walker)

	// Compute hash
	hash, err := hasher.ComputeInputHash(task, nil, tmpDir)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)

	// Verify that changing a matched file changes the hash
	err = os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte("new content"), 0o600)
	require.NoError(t, err)

	newHash, err := hasher.ComputeInputHash(task, nil, tmpDir)
	require.NoError(t, err)
	assert.NotEqual(t, hash, newHash)

	// Verify that changing an unmatched file does NOT change the hash
	err = os.WriteFile(filepath.Join(tmpDir, "c.log"), []byte("new content"), 0o600)
	require.NoError(t, err)

	finalHash, err := hasher.ComputeInputHash(task, nil, tmpDir)
	require.NoError(t, err)
	assert.Equal(t, newHash, finalHash)
}

func TestHasher_ComputeInputHash_MissingFile(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Define task with missing input
	task := &domain.Task{
		Name:   domain.NewInternedString("test-task"),
		Inputs: []domain.InternedString{domain.NewInternedString("missing.txt")},
	}

	// Initialize Hasher
	walker := fs.NewWalker()
	hasher := fs.NewHasher(walker)

	// Compute hash should fail
	_, err := hasher.ComputeInputHash(task, nil, tmpDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "input not found")
}

func TestHasher_ComputeInputHash_WithEnvironment(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test file
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "input.txt"), []byte("content"), 0o600))

	task := &domain.Task{
		Name:   domain.NewInternedString("test-task"),
		Inputs: []domain.InternedString{domain.NewInternedString("input.txt")},
	}

	walker := fs.NewWalker()
	hasher := fs.NewHasher(walker)

	// Compute hash with no env
	hashNoEnv, err := hasher.ComputeInputHash(task, nil, tmpDir)
	require.NoError(t, err)

	// Compute hash with env vars
	env := map[string]string{
		"FOO": "bar",
		"BAZ": "qux",
	}
	hashWithEnv, err := hasher.ComputeInputHash(task, env, tmpDir)
	require.NoError(t, err)

	// Hashes should be different
	assert.NotEqual(t, hashNoEnv, hashWithEnv)

	// Same env should produce same hash
	hashWithEnv2, err := hasher.ComputeInputHash(task, env, tmpDir)
	require.NoError(t, err)
	assert.Equal(t, hashWithEnv, hashWithEnv2)
}

func TestHasher_ComputeInputHash_WithDependencies(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test file
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "input.txt"), []byte("content"), 0o600))

	taskNoDeps := &domain.Task{
		Name:   domain.NewInternedString("test-task"),
		Inputs: []domain.InternedString{domain.NewInternedString("input.txt")},
	}

	taskWithDeps := &domain.Task{
		Name:         domain.NewInternedString("test-task"),
		Inputs:       []domain.InternedString{domain.NewInternedString("input.txt")},
		Dependencies: []domain.InternedString{domain.NewInternedString("dep1"), domain.NewInternedString("dep2")},
	}

	walker := fs.NewWalker()
	hasher := fs.NewHasher(walker)

	hashNoDeps, err := hasher.ComputeInputHash(taskNoDeps, nil, tmpDir)
	require.NoError(t, err)

	hashWithDeps, err := hasher.ComputeInputHash(taskWithDeps, nil, tmpDir)
	require.NoError(t, err)

	// Hashes should be different
	assert.NotEqual(t, hashNoDeps, hashWithDeps)
}

func TestHasher_ComputeInputHash_WithDirectoryInput(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a directory with files
	srcDir := filepath.Join(tmpDir, "src")
	require.NoError(t, os.MkdirAll(srcDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "file1.go"), []byte("package main"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "file2.go"), []byte("func main()"), 0o600))

	task := &domain.Task{
		Name:   domain.NewInternedString("test-task"),
		Inputs: []domain.InternedString{domain.NewInternedString("src")},
	}

	walker := fs.NewWalker()
	hasher := fs.NewHasher(walker)

	hash, err := hasher.ComputeInputHash(task, nil, tmpDir)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)

	// Modify a file in the directory
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "file1.go"), []byte("package main\n// modified"), 0o600))

	newHash, err := hasher.ComputeInputHash(task, nil, tmpDir)
	require.NoError(t, err)

	// Hash should change
	assert.NotEqual(t, hash, newHash)
}

func TestHasher_ComputeInputHash_WithOutputs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test file
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "input.txt"), []byte("content"), 0o600))

	taskNoOutputs := &domain.Task{
		Name:   domain.NewInternedString("test-task"),
		Inputs: []domain.InternedString{domain.NewInternedString("input.txt")},
	}

	taskWithOutputs := &domain.Task{
		Name:    domain.NewInternedString("test-task"),
		Inputs:  []domain.InternedString{domain.NewInternedString("input.txt")},
		Outputs: []domain.InternedString{domain.NewInternedString("output.txt")},
	}

	walker := fs.NewWalker()
	hasher := fs.NewHasher(walker)

	hashNoOutputs, err := hasher.ComputeInputHash(taskNoOutputs, nil, tmpDir)
	require.NoError(t, err)

	hashWithOutputs, err := hasher.ComputeInputHash(taskWithOutputs, nil, tmpDir)
	require.NoError(t, err)

	// Hashes should be different
	assert.NotEqual(t, hashNoOutputs, hashWithOutputs)
}

func TestHasher_ComputeOutputHash(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files
	files := []string{"out1.txt", "out2.txt"}
	for _, f := range files {
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, f), []byte("content"), 0o600))
	}

	walker := fs.NewWalker()
	hasher := fs.NewHasher(walker)

	// Compute hash
	hash, err := hasher.ComputeOutputHash(files, tmpDir)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)

	// Verify order independence (since we sort)
	filesReversed := []string{"out2.txt", "out1.txt"}
	hashReversed, err := hasher.ComputeOutputHash(filesReversed, tmpDir)
	require.NoError(t, err)
	assert.Equal(t, hash, hashReversed)

	// Verify content change changes hash
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "out1.txt"), []byte("new content"), 0o600))
	newHash, err := hasher.ComputeOutputHash(files, tmpDir)
	require.NoError(t, err)
	assert.NotEqual(t, hash, newHash)

	// Verify missing file error
	missingFiles := []string{"out1.txt", "missing.txt"}
	_, err = hasher.ComputeOutputHash(missingFiles, tmpDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "output file missing")
}
