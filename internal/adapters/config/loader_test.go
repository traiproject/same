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
project: "root"
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
	if order[0] != "root:lint" {
		t.Errorf("expected first task to be root:lint, got %s", order[0])
	}
	if order[1] != "root:build" {
		t.Errorf("expected second task to be root:build, got %s", order[1])
	}
}

func TestLoad_MissingDependency(t *testing.T) {
	content := `
version: "1"
project: "root"
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
	if dep, ok := meta["missing_dependency"].(string); !ok || dep != "root:missing" {
		t.Errorf("expected metadata missing_dependency=root:missing, got %v", meta["missing_dependency"])
	}
}

func TestLoad_ReservedTaskName(t *testing.T) {
	content := `
version: "1"
project: "root"
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
project: "root"
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
project: "root"
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
	require.Contains(t, tasks, "root:build")
	buildEnv := tasks["root:build"]
	assert.Equal(t, "0", buildEnv["CGO_ENABLED"])
	assert.Equal(t, "linux", buildEnv["GOOS"])
	assert.Equal(t, "amd64", buildEnv["GOARCH"])
	assert.Len(t, buildEnv, 3)

	// Verify test task environment
	require.Contains(t, tasks, "root:test")
	testEnv := tasks["root:test"]
	assert.Equal(t, "1", testEnv["GO_TEST_VERBOSE"])
	assert.Len(t, testEnv, 1)
}

func TestLoad_WithRoot(t *testing.T) {
	t.Run("Explicit Root", func(t *testing.T) {
		content := `
version: "1"
project: "root"
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
project: "root"
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
project: "root"
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
project: "root"
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
project: "root"
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
	require.Contains(t, tasks, "root:build")
	assert.Equal(t, "/custom/path", tasks["root:build"])

	// Verify test task uses default workingDir (tmpDir)
	require.Contains(t, tasks, "root:test")
	expectedRoot, err := filepath.EvalSymlinks(tmpDir)
	require.NoError(t, err)
	actualRoot, err := filepath.EvalSymlinks(tasks["root:test"])
	require.NoError(t, err)
	assert.Equal(t, expectedRoot, actualRoot)
}

func TestLoad_Workspace(t *testing.T) {
	// Create a temporary directory structure:
	// root/
	//   bob.yaml (workspace: ["packages/*"])
	//   packages/
	//     pkg-a/
	//       bob.yaml
	//     pkg-b/
	//       bob.yaml
	tmpDir := t.TempDir()

	// Root config
	rootContent := `
version: "1"
project: "root"
workspace: ["packages/*"]
tasks:
  root-task:
    cmd: ["echo root"]
`
	err := os.WriteFile(filepath.Join(tmpDir, "bob.yaml"), []byte(rootContent), 0o600)
	require.NoError(t, err)

	// Packages directory
	packagesDir := filepath.Join(tmpDir, "packages")
	err = os.MkdirAll(packagesDir, 0o750)
	require.NoError(t, err)

	// Package A
	pkgADir := filepath.Join(packagesDir, "pkg-a")
	err = os.MkdirAll(pkgADir, 0o750)
	require.NoError(t, err)
	pkgAContent := `
version: "1"
project: "pkg-a"
tasks:
  pkg-a-task:
    cmd: ["echo pkg-a"]
`
	err = os.WriteFile(filepath.Join(pkgADir, "bob.yaml"), []byte(pkgAContent), 0o600)
	require.NoError(t, err)

	// Package B
	pkgBDir := filepath.Join(packagesDir, "pkg-b")
	err = os.MkdirAll(pkgBDir, 0o750)
	require.NoError(t, err)
	pkgBContent := `
version: "1"
project: "pkg-b"
tasks:
  pkg-b-task:
    cmd: ["echo pkg-b"]
`
	err = os.WriteFile(filepath.Join(pkgBDir, "bob.yaml"), []byte(pkgBContent), 0o600)
	require.NoError(t, err)

	// Load the config
	g, err := config.Load(filepath.Join(tmpDir, "bob.yaml"))
	require.NoError(t, err)

	// Validate the graph to populate execution order
	err = g.Validate()
	require.NoError(t, err)

	// Verify tasks
	tasks := make(map[string]string)
	for task := range g.Walk() {
		tasks[task.Name.String()] = task.WorkingDir.String()
	}

	// Verify root task
	require.Contains(t, tasks, "root:root-task")
	rootPath, err := filepath.EvalSymlinks(tmpDir)
	require.NoError(t, err)
	rootTaskPath, err := filepath.EvalSymlinks(tasks["root:root-task"])
	require.NoError(t, err)
	assert.Equal(t, rootPath, rootTaskPath)

	// Verify pkg-a task
	require.Contains(t, tasks, "pkg-a:pkg-a-task")
	pkgAPath, err := filepath.EvalSymlinks(pkgADir)
	require.NoError(t, err)
	pkgATaskPath, err := filepath.EvalSymlinks(tasks["pkg-a:pkg-a-task"])
	require.NoError(t, err)
	assert.Equal(t, pkgAPath, pkgATaskPath)

	// Verify pkg-b task
	require.Contains(t, tasks, "pkg-b:pkg-b-task")
	pkgBPath, err := filepath.EvalSymlinks(pkgBDir)
	require.NoError(t, err)
	pkgBTaskPath, err := filepath.EvalSymlinks(tasks["pkg-b:pkg-b-task"])
	require.NoError(t, err)
	assert.Equal(t, pkgBPath, pkgBTaskPath)
}

func TestLoad_MissingProjectName(t *testing.T) {
	content := `
version: "1"
tasks:
  build:
    cmd: ["echo build"]
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "bob.yaml")
	err := os.WriteFile(configPath, []byte(content), 0o600)
	require.NoError(t, err)

	_, err = config.Load(configPath)
	require.Error(t, err)

	zErr, ok := err.(*zerr.Error)
	if !ok {
		t.Fatalf("expected *zerr.Error, got %T: %v", err, err)
	}
	meta := zErr.Metadata()
	if msg, ok := meta["error"].(string); !ok || msg != "missing project name" {
		t.Errorf("expected metadata error='missing project name', got %v", meta["error"])
	}
}

func TestLoad_DuplicateProjectName(t *testing.T) {
	tmpDir := t.TempDir()

	// Root config
	rootContent := `
version: "1"
project: "common"
workspace: ["pkg"]
tasks: {}
`
	err := os.WriteFile(filepath.Join(tmpDir, "bob.yaml"), []byte(rootContent), 0o600)
	require.NoError(t, err)

	// Package config with same project name
	pkgDir := filepath.Join(tmpDir, "pkg")
	err = os.MkdirAll(pkgDir, 0o750)
	require.NoError(t, err)
	pkgContent := `
version: "1"
project: "common"
tasks: {}
`
	err = os.WriteFile(filepath.Join(pkgDir, "bob.yaml"), []byte(pkgContent), 0o600)
	require.NoError(t, err)

	_, err = config.Load(filepath.Join(tmpDir, "bob.yaml"))
	require.Error(t, err)

	zErr, ok := err.(*zerr.Error)
	if !ok {
		t.Fatalf("expected *zerr.Error, got %T: %v", err, err)
	}
	meta := zErr.Metadata()
	if msg, ok := meta["error"].(string); !ok || msg != "duplicate project name" {
		t.Errorf("expected metadata error='duplicate project name', got %v", meta["error"])
	}
}

func TestLoad_ReservedProjectName(t *testing.T) {
	content := `
version: "1"
project: "all"
tasks:
  build:
    cmd: ["echo build"]
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "bob.yaml")
	err := os.WriteFile(configPath, []byte(content), 0o600)
	require.NoError(t, err)

	_, err = config.Load(configPath)
	require.Error(t, err)

	zErr, ok := err.(*zerr.Error)
	if !ok {
		t.Fatalf("expected *zerr.Error, got %T: %v", err, err)
	}
	meta := zErr.Metadata()
	if project, ok := meta["project"].(string); !ok || project != "all" {
		t.Errorf("expected metadata project='all', got %v", meta["project"])
	}
}

func TestLoad_CrossProjectDependency(t *testing.T) {
	tmpDir := t.TempDir()

	// Root config
	rootContent := `
version: "1"
project: "root"
workspace: ["lib"]
tasks:
  build:
    cmd: ["echo root build"]
    dependsOn: ["lib:build"]
`
	err := os.WriteFile(filepath.Join(tmpDir, "bob.yaml"), []byte(rootContent), 0o600)
	require.NoError(t, err)

	// Lib config
	libDir := filepath.Join(tmpDir, "lib")
	err = os.MkdirAll(libDir, 0o750)
	require.NoError(t, err)
	libContent := `
version: "1"
project: "lib"
tasks:
  build:
    cmd: ["echo lib build"]
`
	err = os.WriteFile(filepath.Join(libDir, "bob.yaml"), []byte(libContent), 0o600)
	require.NoError(t, err)

	g, err := config.Load(filepath.Join(tmpDir, "bob.yaml"))
	require.NoError(t, err)

	// Validate to populate execution order
	err = g.Validate()
	require.NoError(t, err)

	// Verify dependency
	tasks := make(map[string][]string)
	for task := range g.Walk() {
		deps := make([]string, len(task.Dependencies))
		for i, dep := range task.Dependencies {
			deps[i] = dep.String()
		}
		tasks[task.Name.String()] = deps
	}

	require.Contains(t, tasks, "root:build")
	assert.Contains(t, tasks["root:build"], "lib:build")
}

func TestLoad_LocalDependency(t *testing.T) {
	content := `
version: "1"
project: "myproj"
tasks:
  build:
    cmd: ["echo build"]
    dependsOn: ["lint"]
  lint:
    cmd: ["echo lint"]
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "bob.yaml")
	err := os.WriteFile(configPath, []byte(content), 0o600)
	require.NoError(t, err)

	g, err := config.Load(configPath)
	require.NoError(t, err)

	// Validate to populate execution order
	err = g.Validate()
	require.NoError(t, err)

	tasks := make(map[string][]string)
	for task := range g.Walk() {
		deps := make([]string, len(task.Dependencies))
		for i, dep := range task.Dependencies {
			deps[i] = dep.String()
		}
		tasks[task.Name.String()] = deps
	}

	require.Contains(t, tasks, "myproj:build")
	assert.Contains(t, tasks["myproj:build"], "myproj:lint")
}

func TestLoad_InvalidDependency(t *testing.T) {
	content := `
version: "1"
project: "myproj"
tasks:
  build:
    cmd: ["echo build"]
    dependsOn: ["missing"]
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "bob.yaml")
	err := os.WriteFile(configPath, []byte(content), 0o600)
	require.NoError(t, err)

	_, err = config.Load(configPath)
	require.Error(t, err)

	// Verify error metadata
	zErr, ok := err.(*zerr.Error)
	if !ok {
		t.Fatalf("expected *zerr.Error, got %T: %v", err, err)
	}
	meta := zErr.Metadata()
	if dep, ok := meta["missing_dependency"].(string); !ok || dep != "myproj:missing" {
		t.Errorf("expected metadata missing_dependency=myproj:missing, got %v", meta["missing_dependency"])
	}
}

func TestLoad_InvalidProjectName(t *testing.T) {
	t.Run("Invalid Characters", func(t *testing.T) {
		content := `
version: "1"
project: "my project"
tasks: {}
`
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "bob.yaml")
		err := os.WriteFile(configPath, []byte(content), 0o600)
		require.NoError(t, err)

		_, err = config.Load(configPath)
		require.Error(t, err)

		zErr, ok := err.(*zerr.Error)
		require.True(t, ok)
		meta := zErr.Metadata()
		assert.Equal(t, "project name must can only contain alphanumeric characters, underscores or hyphens", meta["error"])
		assert.Equal(t, "my project", meta["project"])
	})
}
