package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRun(t *testing.T) {
	// Save original args
	originalArgs := os.Args
	defer func() {
		os.Args = originalArgs
	}()

	tests := []struct {
		name         string
		setupConfig  func(tmpDir string) string
		args         []string
		expectedExit int
	}{
		{
			name: "Success with valid config",
			setupConfig: func(tmpDir string) string {
				configPath := tmpDir + "/bob.yaml"
				configContent := `version: "1"
project: "test"
tasks:
  test:
    cmd: ["echo", "hello"]
`
				err := os.WriteFile(configPath, []byte(configContent), 0o600)
				if err != nil {
					t.Fatalf("failed to write config: %v", err)
				}
				return configPath
			},
			args:         []string{"bob", "run", "test:test"},
			expectedExit: 0,
		},
		{
			name: "Error with missing config",
			setupConfig: func(tmpDir string) string {
				return tmpDir + "/nonexistent.yaml"
			},
			args:         []string{"bob", "-c", "nonexistent.yaml", "run", "test"},
			expectedExit: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Setup config
			configPath := tt.setupConfig(tmpDir)

			// Change to tmpDir for relative path resolution
			originalWd, _ := os.Getwd()
			err := os.Chdir(tmpDir)
			if err != nil {
				t.Fatalf("failed to chdir: %v", err)
			}
			defer func() {
				_ = os.Chdir(originalWd)
			}()

			// Set args
			os.Args = tt.args
			if tt.args[1] == "-c" {
				os.Args[2] = configPath
			}

			// Run and capture exit code
			exitCode := run()
			assert.Equal(t, tt.expectedExit, exitCode)
		})
	}
}

func TestRun_StoreInitError(t *testing.T) {
	// Save original args
	originalArgs := os.Args
	defer func() {
		os.Args = originalArgs
	}()

	tmpDir := t.TempDir()

	// Create a valid config
	configPath := tmpDir + "/bob.yaml"
	configContent := `version: "1"
project: "test"
tasks:
  test:
    cmd: ["echo", "hello"]
`
	err := os.WriteFile(configPath, []byte(configContent), 0o600)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Create .bob directory as a file (not a directory) to cause store init to fail
	bobPath := tmpDir + "/.bob"
	err = os.WriteFile(bobPath, []byte("not a directory"), 0o600)
	if err != nil {
		t.Fatalf("failed to create .bob file: %v", err)
	}

	// Change to tmpDir
	originalWd, _ := os.Getwd()
	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer func() {
		_ = os.Chdir(originalWd)
	}()

	// Set args
	os.Args = []string{"bob", "run", "test:test"}

	// Run and expect error exit code
	exitCode := run()
	assert.Equal(t, 1, exitCode)
}
