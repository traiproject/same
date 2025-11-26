package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseConfigFlag(t *testing.T) {
	// Save original args and defer restoration
	originalArgs := os.Args
	defer func() {
		os.Args = originalArgs
	}()

	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "Default behavior (no flags)",
			args:     []string{"bob"},
			expected: "bob.yaml",
		},
		{
			name:     "Short flag",
			args:     []string{"bob", "-c", "custom.yaml"},
			expected: "custom.yaml",
		},
		{
			name:     "Long flag",
			args:     []string{"bob", "--config", "custom.yaml"},
			expected: "custom.yaml",
		},
		{
			name:     "Equals format",
			args:     []string{"bob", "--config=custom.yaml"},
			expected: "custom.yaml",
		},
		{
			name:     "Flag at the end",
			args:     []string{"bob", "run", "build", "-c", "custom.yaml"},
			expected: "custom.yaml",
		},
		{
			name:     "Flag in the middle",
			args:     []string{"bob", "-c", "custom.yaml", "run"},
			expected: "custom.yaml",
		},
		{
			name:     "Flag present but no value (edge case)",
			args:     []string{"bob", "-c"},
			expected: "bob.yaml",
		},
		{
			name:     "Long flag present but no value (edge case)",
			args:     []string{"bob", "--config"},
			expected: "bob.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Args = tt.args
			got := parseConfigFlag()
			assert.Equal(t, tt.expected, got)
		})
	}
}

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
