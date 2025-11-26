package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun_Integration(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	bobYamlPath := filepath.Join(tempDir, "bob.yaml")
	err := os.WriteFile(bobYamlPath, []byte("version: \"1\"\ntasks: { build: { cmd: [\"echo\", \"hello\"] } }\n"), 0o600)
	require.NoError(t, err)

	originalWd, err := os.Getwd()
	require.NoError(t, err)
	originalArgs := os.Args
	defer func() {
		_ = os.Chdir(originalWd)
		os.Args = originalArgs
	}()

	err = os.Chdir(tempDir)
	require.NoError(t, err)

	// Set args to run the 'build' task
	os.Args = []string{"bob", "run", "build"}

	// Execution
	exitCode := run()

	// Assertion
	assert.Equal(t, 0, exitCode)
}

func TestRun_MissingConfig(t *testing.T) {
	// Setup
	tempDir := t.TempDir()

	originalWd, err := os.Getwd()
	require.NoError(t, err)
	originalArgs := os.Args
	defer func() {
		_ = os.Chdir(originalWd)
		os.Args = originalArgs
	}()

	err = os.Chdir(tempDir)
	require.NoError(t, err)

	// Set args to run a task, which should trigger config load
	os.Args = []string{"bob", "run", "build"}

	// Execution
	exitCode := run()

	// Assertion
	assert.Equal(t, 1, exitCode)
}
