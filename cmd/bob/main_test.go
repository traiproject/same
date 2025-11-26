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
			args:         []string{"bob", "run", "test"},
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
