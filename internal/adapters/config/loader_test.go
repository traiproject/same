package config_test

import (
	"os"
	"path/filepath"
	"strings"
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
	if !strings.Contains(err.Error(), "reserved") {
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

func TestLoad_WithEnvironment(t *testing.T) {
	// Create a config file with environment variables
	content := `
version: "1"
tasks:
  build:
    cmd: ["go", "build"]
    environment:
      CGO_ENABLED: "0"
      GOOS: "linux"
      GOARCH: "amd64"
  test:
    cmd: ["go", "test"]
    environment:
      GO_TEST_VERBOSE: "1"
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "bob.yaml")
	err := os.WriteFile(configPath, []byte(content), 0o600)
	require.NoError(t, err)

	// Load the config
	g, err := config.Load(configPath)
	require.NoError(t, err)

	// Verify graph is valid
	err = g.Validate()
	require.NoError(t, err)

	// Collect tasks and verify environment
	tasks := make(map[string]map[string]string)
	for task := range g.Walk() {
		tasks[task.Name.String()] = task.Environment
	}

	// Verify build task environment
	require.Contains(t, tasks, "build")
	buildEnv := tasks["build"]
	assert.Equal(t, "0", buildEnv["CGO_ENABLED"])
	assert.Equal(t, "linux", buildEnv["GOOS"])
	assert.Equal(t, "amd64", buildEnv["GOARCH"])
	assert.Len(t, buildEnv, 3)

	// Verify test task environment
	require.Contains(t, tasks, "test")
	testEnv := tasks["test"]
	assert.Equal(t, "1", testEnv["GO_TEST_VERBOSE"])
	assert.Len(t, testEnv, 1)
}

func TestLoad_WithRoot(t *testing.T) {
	t.Run("Explicit Root", func(t *testing.T) {
		content := `
version: "1"
root: "./src"
tasks: {}
`
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "bob.yaml")
		err := os.WriteFile(configPath, []byte(content), 0o600)
		require.NoError(t, err)

		g, err := config.Load(configPath)
		require.NoError(t, err)

		expectedRoot := filepath.Join(tmpDir, "src")
		assert.Equal(t, expectedRoot, g.Root())
	})

	t.Run("Implicit Root", func(t *testing.T) {
		content := `
version: "1"
tasks: {}
`
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "bob.yaml")
		err := os.WriteFile(configPath, []byte(content), 0o600)
		require.NoError(t, err)

		g, err := config.Load(configPath)
		require.NoError(t, err)

		// On some platforms (e.g., macOS), t.TempDir() may return a symlinked path.
		// To ensure the comparison is robust, resolve symlinks in both paths before comparing.
		expectedRoot, err := filepath.EvalSymlinks(tmpDir)
		require.NoError(t, err)
		actualRoot, err := filepath.EvalSymlinks(g.Root())
		require.NoError(t, err)
		assert.Equal(t, expectedRoot, actualRoot)
	})

	t.Run("Absolute Root", func(t *testing.T) {
		tmpDir := t.TempDir()
		absoluteRoot := filepath.Join(tmpDir, "absolute-root")

		content := `
version: "1"
root: "` + absoluteRoot + `"
tasks: {}
`
		configPath := filepath.Join(tmpDir, "bob.yaml")
		err := os.WriteFile(configPath, []byte(content), 0o600)
		require.NoError(t, err)

		g, err := config.Load(configPath)
		require.NoError(t, err)

		assert.Equal(t, absoluteRoot, g.Root())
	})
}

func TestFileConfigLoader_Load(t *testing.T) {
	content := `
version: "1"
tasks:
  build:
    cmd: ["go", "build"]
    input: ["*.go"]
  test:
    cmd: ["go", "test"]
    dependsOn: ["build"]
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "bob.yaml")
	err := os.WriteFile(configPath, []byte(content), 0o600)
	require.NoError(t, err)

	loader := &config.FileConfigLoader{Filename: "bob.yaml"}
	g, err := loader.Load(tmpDir)
	require.NoError(t, err)
	require.NotNil(t, g)

	// Verify the graph is valid
	err = g.Validate()
	require.NoError(t, err)

	// Verify tasks were loaded
	taskCount := 0
	for range g.Walk() {
		taskCount++
	}
	assert.Equal(t, 2, taskCount)
}

func TestLoad_WithWorkingDir(t *testing.T) {
	content := `
version: "1"
tasks:
  build:
    cmd: ["go", "build"]
    workingDir: "/custom/path"
  test:
    cmd: ["go", "test"]
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "bob.yaml")
	err := os.WriteFile(configPath, []byte(content), 0o600)
	require.NoError(t, err)

	g, err := config.Load(configPath)
	require.NoError(t, err)

	// Verify graph is valid
	err = g.Validate()
	require.NoError(t, err)

	// Collect tasks and verify workingDir
	tasks := make(map[string]string)
	for task := range g.Walk() {
		tasks[task.Name.String()] = task.WorkingDir.String()
	}

	// Verify build task has custom workingDir
	require.Contains(t, tasks, "build")
	assert.Equal(t, "/custom/path", tasks["build"])

	// Verify test task uses default workingDir (tmpDir)
	require.Contains(t, tasks, "test")
	expectedRoot, err := filepath.EvalSymlinks(tmpDir)
	require.NoError(t, err)
	actualRoot, err := filepath.EvalSymlinks(tasks["test"])
	require.NoError(t, err)
	assert.Equal(t, expectedRoot, actualRoot)
}
