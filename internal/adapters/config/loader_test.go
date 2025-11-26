package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/bob/internal/adapters/config"
	"go.trai.ch/zerr"
)

func TestLoad_Success(t *testing.T) {
	// Create a temporary config file
	content := `
version: "1"
tasks:
  build:
    input: ["src/**/*"]
    cmd: ["go build"]
    target: ["bin/app"]
    dependsOn: ["lint"]
  lint:
    cmd: ["golangci-lint run"]
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "bob.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Load the config
	g, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify graph structure
	// Since we can't inspect private fields of Graph easily without helpers,
	// we can use Walk or Validate to check.
	// Or we can just check if Validate passes.
	if err := g.Validate(); err != nil {
		t.Fatalf("graph validation failed: %v", err)
	}

	// Verify execution order (lint -> build)
	order := make([]string, 0, 2)
	for task := range g.Walk() {
		order = append(order, task.Name.String())
	}

	if len(order) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(order))
	}
	if order[0] != "lint" {
		t.Errorf("expected first task to be lint, got %s", order[0])
	}
	if order[1] != "build" {
		t.Errorf("expected second task to be build, got %s", order[1])
	}
}

func TestLoad_MissingDependency(t *testing.T) {
	content := `
version: "1"
tasks:
  build:
    dependsOn: ["missing"]
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "bob.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Load the config
	_, err := config.Load(configPath)
	if err == nil {
		t.Fatal("expected error for missing dependency, got nil")
	}

	// Verify error
	zErr, ok := err.(*zerr.Error)
	if !ok {
		t.Fatalf("expected *zerr.Error, got %T: %v", err, err)
	}

	meta := zErr.Metadata()
	if dep, ok := meta["missing_dependency"].(string); !ok || dep != "missing" {
		t.Errorf("expected metadata missing_dependency=missing, got %v", meta["missing_dependency"])
	}
}

func TestLoad_ReservedTaskName(t *testing.T) {
	content := `
version: "1"
tasks:
  all:
    cmd: ["echo hello"]
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "bob.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Load the config
	_, err := config.Load(configPath)
	if err == nil {
		t.Fatal("expected error for reserved task name 'all', got nil")
	}

	// Verify error message contains "reserved"
	if !contains(err.Error(), "reserved") {
		t.Errorf("expected error message to contain 'reserved', got: %v", err)
	}

	// Verify error metadata
	zErr, ok := err.(*zerr.Error)
	if !ok {
		t.Fatalf("expected *zerr.Error, got %T: %v", err, err)
	}

	meta := zErr.Metadata()
	if taskName, ok := meta["task_name"].(string); !ok || taskName != "all" {
		t.Errorf("expected metadata task_name=all, got %v", meta["task_name"])
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && func() bool {
		for i := 0; i <= len(s)-len(substr); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	}())
}

func TestLoad_Errors(t *testing.T) {
	t.Run("File Not Found", func(t *testing.T) {
		_, err := config.Load("non-existent-file.yaml")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read config file")
	})

	t.Run("Invalid YAML", func(t *testing.T) {
		content := `
version: "1"
tasks:
  build:
    cmd: ["echo hello"]
    input: ["src/**/*"  # Unclosed list/quote
`
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "invalid.yaml")
		err := os.WriteFile(configPath, []byte(content), 0o600)
		require.NoError(t, err)

		_, err = config.Load(configPath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse config file")
	})
}
