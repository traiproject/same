package main

import (
	"io"
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"go.trai.ch/same/internal/app"
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
				configPath := tmpDir + "/same.yaml"
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
			args:         []string{"same", "run", "test"},
			expectedExit: 0,
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
			exitCode := run(func(a *app.App) {
				a.WithTeaOptions(tea.WithInput(nil), tea.WithOutput(io.Discard))
			})
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
	configPath := tmpDir + "/same.yaml"
	configContent := `version: "1"
tasks:
  test:
    cmd: ["echo", "hello"]
`
	err := os.WriteFile(configPath, []byte(configContent), 0o600)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Create .same directory as a file (not a directory) to cause store init to fail
	samePath := tmpDir + "/.same"
	err = os.WriteFile(samePath, []byte("not a directory"), 0o600)
	if err != nil {
		t.Fatalf("failed to create .same file: %v", err)
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
	os.Args = []string{"same", "run", "test"}

	// Run and expect error exit code
	exitCode := run()
	assert.Equal(t, 1, exitCode)
}
