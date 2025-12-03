package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/bob/internal/adapters/config"
	"go.trai.ch/bob/internal/core/ports/mocks"
	"go.trai.ch/zerr"
	"go.uber.org/mock/gomock"
)

func TestLoad_DuplicateProjectName(t *testing.T) {
	tmpDir := t.TempDir()

	// Create workspace config
	workfileContent := `
version: "1"
projects:
  - "packages/*"
`
	workfilePath := filepath.Join(tmpDir, "bob.work.yaml")
	err := os.WriteFile(workfilePath, []byte(workfileContent), 0o600)
	require.NoError(t, err)

	// Create packages directory
	packagesDir := filepath.Join(tmpDir, "packages")
	err = os.MkdirAll(packagesDir, 0o750)
	require.NoError(t, err)

	// Create first project with name "myapp"
	project1Dir := filepath.Join(packagesDir, "project1")
	err = os.MkdirAll(project1Dir, 0o750)
	require.NoError(t, err)
	bobfile1Content := `
version: "1"
project: "myapp"
tasks:
  build:
    cmd: ["echo", "building project1"]
`
	bobfile1Path := filepath.Join(project1Dir, "bob.yaml")
	err = os.WriteFile(bobfile1Path, []byte(bobfile1Content), 0o600)
	require.NoError(t, err)

	// Create second project with the same name "myapp"
	project2Dir := filepath.Join(packagesDir, "project2")
	err = os.MkdirAll(project2Dir, 0o750)
	require.NoError(t, err)
	bobfile2Content := `
version: "1"
project: "myapp"
tasks:
  test:
    cmd: ["echo", "testing project2"]
`
	bobfile2Path := filepath.Join(project2Dir, "bob.yaml")
	err = os.WriteFile(bobfile2Path, []byte(bobfile2Content), 0o600)
	require.NoError(t, err)

	// Load the config
	ctrl := gomock.NewController(t)
	mockLogger := mocks.NewMockLogger(ctrl)
	loader := &config.Loader{Logger: mockLogger}

	_, err = loader.Load(tmpDir)
	require.Error(t, err, "expected error for duplicate project name")

	// Verify it's the correct error type
	zErr, ok := err.(*zerr.Error)
	require.True(t, ok, "expected *zerr.Error, got %T: %v", err, err)

	// Check error is domain.ErrDuplicateProjectName
	assert.Contains(t, err.Error(), "duplicate project name")

	// Verify error metadata
	meta := zErr.Metadata()
	assert.Equal(t, "myapp", meta["project_name"], "expected project_name metadata")
	assert.Contains(t, meta, "first_occurrence", "expected first_occurrence metadata")
	assert.Contains(t, meta, "duplicate_at", "expected duplicate_at metadata")

	// The order of processing depends on sorting, so we need to check both possibilities
	first := meta["first_occurrence"].(string)
	duplicate := meta["duplicate_at"].(string)

	assert.True(t,
		(first == "packages/project1" && duplicate == "packages/project2") ||
			(first == "packages/project2" && duplicate == "packages/project1"),
		"expected first_occurrence and duplicate_at to be packages/project1 and packages/project2 in some order")
}
