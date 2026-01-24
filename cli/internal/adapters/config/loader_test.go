package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/same/internal/adapters/config"
	"go.trai.ch/same/internal/core/domain"
	"go.trai.ch/same/internal/core/ports/mocks"
	"go.trai.ch/zerr"
	"go.uber.org/mock/gomock"
)

const minimalWorkfileYAML = `
version: "1"
projects:
  - "project-a"
`

func TestNewLoader(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockLogger := mocks.NewMockLogger(ctrl)

	loader := config.NewLoader(mockLogger)
	require.NotNil(t, loader)
	require.Equal(t, mockLogger, loader.Logger)
}

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
	configPath := filepath.Join(tmpDir, "same.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Load the config
	ctrl := gomock.NewController(t)
	loader := &config.Loader{Logger: mocks.NewMockLogger(ctrl)}
	g, err := loader.Load(tmpDir)
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
	configPath := filepath.Join(tmpDir, "same.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Load the config
	ctrl := gomock.NewController(t)
	loader := &config.Loader{Logger: mocks.NewMockLogger(ctrl)}
	_, err := loader.Load(tmpDir)
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
	configPath := filepath.Join(tmpDir, "same.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Load the config
	ctrl := gomock.NewController(t)
	loader := &config.Loader{Logger: mocks.NewMockLogger(ctrl)}
	_, err := loader.Load(tmpDir)
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

func TestLoad_InvalidTaskName(t *testing.T) {
	content := `
version: "1"
tasks:
  invalid:name:
    cmd: ["echo hello"]
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "same.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Load the config
	ctrl := gomock.NewController(t)
	loader := &config.Loader{Logger: mocks.NewMockLogger(ctrl)}
	_, err := loader.Load(tmpDir)
	if err == nil {
		t.Fatal("expected error for task name with colon, got nil")
	}

	// Verify error message contains "invalid"
	if !strings.Contains(err.Error(), "invalid") {
		t.Errorf("expected error message to contain 'invalid', got: %v", err)
	}

	// Verify error metadata
	zErr, ok := err.(*zerr.Error)
	if !ok {
		t.Fatalf("expected *zerr.Error, got %T: %v", err, err)
	}

	meta := zErr.Metadata()
	if invalidChar, ok := meta["invalid_character"].(string); !ok || invalidChar != ":" {
		t.Errorf("expected metadata invalid_character=':' got %v", meta["invalid_character"])
	}
	if taskName, ok := meta["task_name"].(string); !ok || taskName != "invalid:name" {
		t.Errorf("expected metadata task_name='invalid:name', got %v", meta["task_name"])
	}
}

func TestLoad_Errors(t *testing.T) {
	t.Run("File Not Found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		loader := &config.Loader{Logger: mocks.NewMockLogger(ctrl)}
		_, err := loader.Load("/non-existent-directory")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "could not find samefile or workfile")
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
		configPath := filepath.Join(tmpDir, "same.yaml")
		err := os.WriteFile(configPath, []byte(content), 0o600)
		require.NoError(t, err)

		ctrl := gomock.NewController(t)
		loader := &config.Loader{Logger: mocks.NewMockLogger(ctrl)}
		_, err = loader.Load(tmpDir)
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
	configPath := filepath.Join(tmpDir, "same.yaml")
	err := os.WriteFile(configPath, []byte(content), 0o600)
	require.NoError(t, err)

	// Load the config
	ctrl := gomock.NewController(t)
	loader := &config.Loader{Logger: mocks.NewMockLogger(ctrl)}
	g, err := loader.Load(tmpDir)
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
		configPath := filepath.Join(tmpDir, "same.yaml")
		err := os.WriteFile(configPath, []byte(content), 0o600)
		require.NoError(t, err)

		ctrl := gomock.NewController(t)
		loader := &config.Loader{Logger: mocks.NewMockLogger(ctrl)}
		g, err := loader.Load(tmpDir)
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
		configPath := filepath.Join(tmpDir, "same.yaml")
		err := os.WriteFile(configPath, []byte(content), 0o600)
		require.NoError(t, err)

		ctrl := gomock.NewController(t)
		loader := &config.Loader{Logger: mocks.NewMockLogger(ctrl)}
		g, err := loader.Load(tmpDir)
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
		configPath := filepath.Join(tmpDir, "same.yaml")
		err := os.WriteFile(configPath, []byte(content), 0o600)
		require.NoError(t, err)

		ctrl := gomock.NewController(t)
		loader := &config.Loader{Logger: mocks.NewMockLogger(ctrl)}
		g, err := loader.Load(tmpDir)
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
	configPath := filepath.Join(tmpDir, "same.yaml")
	err := os.WriteFile(configPath, []byte(content), 0o600)
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	loader := &config.Loader{Logger: mocks.NewMockLogger(ctrl)}
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
	configPath := filepath.Join(tmpDir, "same.yaml")
	err := os.WriteFile(configPath, []byte(content), 0o600)
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	loader := &config.Loader{Logger: mocks.NewMockLogger(ctrl)}
	g, err := loader.Load(tmpDir)
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

func TestLoad_ProjectFieldWarningInStandaloneMode(t *testing.T) {
	content := `
version: "1"
project: "my-project"
tasks:
  build:
    cmd: ["go", "build"]
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "same.yaml")
	err := os.WriteFile(configPath, []byte(content), 0o600)
	require.NoError(t, err)

	// Create mock logger to capture warnings
	ctrl := gomock.NewController(t)
	mockLogger := mocks.NewMockLogger(ctrl)

	// Expect the warning to be logged
	mockLogger.EXPECT().
		Warn(gomock.Eq("'project' defined in same.yaml has no effect in standalone mode")).
		Times(1)

	loader := &config.Loader{Logger: mockLogger}
	g, err := loader.Load(tmpDir)
	require.NoError(t, err)
	require.NotNil(t, g)

	// Verify the graph is valid
	err = g.Validate()
	require.NoError(t, err)
}

func TestLoad_WithTools(t *testing.T) {
	content := `
version: "1"
tools:
  go: "go@1.23"
  node: "nodejs@20"
tasks:
  build:
    cmd: ["go", "build"]
    tools: ["go"]
  test:
    cmd: ["go", "test"]
    tools: ["go", "node"]
  lint:
    cmd: ["golangci-lint", "run"]
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "same.yaml")
	err := os.WriteFile(configPath, []byte(content), 0o600)
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	loader := &config.Loader{Logger: mocks.NewMockLogger(ctrl)}
	g, err := loader.Load(tmpDir)
	require.NoError(t, err)

	err = g.Validate()
	require.NoError(t, err)

	// Collect tasks and verify tools
	tasks := make(map[string]map[string]string)
	for task := range g.Walk() {
		tasks[task.Name.String()] = task.Tools
	}

	// Verify build task has only go tool
	require.Contains(t, tasks, "build")
	assert.Equal(t, map[string]string{"go": "go@1.23"}, tasks["build"])

	// Verify test task has both go and node tools
	require.Contains(t, tasks, "test")
	assert.Equal(t, map[string]string{
		"go":   "go@1.23",
		"node": "nodejs@20",
	}, tasks["test"])

	// Verify lint task has no tools
	require.Contains(t, tasks, "lint")
	assert.Nil(t, tasks["lint"])
}

func TestLoad_MissingTool(t *testing.T) {
	content := `
version: "1"
tools:
  go: "go@1.23"
tasks:
  build:
    cmd: ["go", "build"]
    tools: ["go", "undefined-tool"]
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "same.yaml")
	err := os.WriteFile(configPath, []byte(content), 0o600)
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	loader := &config.Loader{Logger: mocks.NewMockLogger(ctrl)}
	_, err = loader.Load(tmpDir)
	require.Error(t, err)

	// Verify error message
	assert.Contains(t, err.Error(), "tool not found")

	// Verify error metadata
	zErr, ok := err.(*zerr.Error)
	require.True(t, ok, "expected *zerr.Error, got %T", err)

	meta := zErr.Metadata()
	assert.Equal(t, "undefined-tool", meta["tool_alias"])
	assert.Equal(t, "build", meta["task"])
}

func TestLoad_RebuildStrategy(t *testing.T) {
	t.Run("Default (empty) to on-change", func(t *testing.T) {
		content := `
version: "1"
tasks:
  build:
    cmd: ["go", "build"]
`
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "same.yaml")
		err := os.WriteFile(configPath, []byte(content), 0o600)
		require.NoError(t, err)

		ctrl := gomock.NewController(t)
		loader := &config.Loader{Logger: mocks.NewMockLogger(ctrl)}
		g, err := loader.Load(tmpDir)
		require.NoError(t, err)

		task, ok := g.GetTask(domain.NewInternedString("build"))
		require.True(t, ok)
		assert.Equal(t, domain.RebuildOnChange, task.RebuildStrategy)
	})

	t.Run("Explicit on-change", func(t *testing.T) {
		content := `
version: "1"
tasks:
  build:
    cmd: ["go", "build"]
    rebuild: "on-change"
`
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "same.yaml")
		err := os.WriteFile(configPath, []byte(content), 0o600)
		require.NoError(t, err)

		ctrl := gomock.NewController(t)
		loader := &config.Loader{Logger: mocks.NewMockLogger(ctrl)}
		g, err := loader.Load(tmpDir)
		require.NoError(t, err)

		task, ok := g.GetTask(domain.NewInternedString("build"))
		require.True(t, ok)
		assert.Equal(t, domain.RebuildOnChange, task.RebuildStrategy)
	})

	t.Run("Always rebuild", func(t *testing.T) {
		content := `
version: "1"
tasks:
  build:
    cmd: ["go", "build"]
    rebuild: "always"
`
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "same.yaml")
		err := os.WriteFile(configPath, []byte(content), 0o600)
		require.NoError(t, err)

		ctrl := gomock.NewController(t)
		loader := &config.Loader{Logger: mocks.NewMockLogger(ctrl)}
		g, err := loader.Load(tmpDir)
		require.NoError(t, err)

		task, ok := g.GetTask(domain.NewInternedString("build"))
		require.True(t, ok)
		assert.Equal(t, domain.RebuildAlways, task.RebuildStrategy)
	})

	t.Run("Invalid rebuild strategy", func(t *testing.T) {
		content := `
version: "1"
tasks:
  build:
    cmd: ["go", "build"]
    rebuild: "invalid"
`
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "same.yaml")
		err := os.WriteFile(configPath, []byte(content), 0o600)
		require.NoError(t, err)

		ctrl := gomock.NewController(t)
		loader := &config.Loader{Logger: mocks.NewMockLogger(ctrl)}
		_, err = loader.Load(tmpDir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid rebuild strategy")
	})
}

func TestLoad_Workfile(t *testing.T) {
	t.Run("Basic workfile with projects", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create workspace structure
		workfileContent := `
version: "1"
projects:
  - "project-a"
  - "project-b"
`
		workfilePath := filepath.Join(tmpDir, "same.work.yaml")
		err := os.WriteFile(workfilePath, []byte(workfileContent), 0o600)
		require.NoError(t, err)

		// Create project-a
		projectADir := filepath.Join(tmpDir, "project-a")
		err = os.Mkdir(projectADir, 0o750)
		require.NoError(t, err)
		samefileA := `
version: "1"
project: "proj-a"
tasks:
  build:
    cmd: ["go", "build"]
`
		err = os.WriteFile(filepath.Join(projectADir, "same.yaml"), []byte(samefileA), 0o600)
		require.NoError(t, err)

		// Create project-b
		projectBDir := filepath.Join(tmpDir, "project-b")
		err = os.Mkdir(projectBDir, 0o750)
		require.NoError(t, err)
		samefileB := `
version: "1"
project: "proj-b"
tasks:
  test:
    cmd: ["go", "test"]
`
		err = os.WriteFile(filepath.Join(projectBDir, "same.yaml"), []byte(samefileB), 0o600)
		require.NoError(t, err)

		ctrl := gomock.NewController(t)
		loader := &config.Loader{Logger: mocks.NewMockLogger(ctrl)}
		g, err := loader.Load(tmpDir)
		require.NoError(t, err)

		err = g.Validate()
		require.NoError(t, err)

		// Verify tasks are namespaced
		_, ok := g.GetTask(domain.NewInternedString("proj-a:build"))
		assert.True(t, ok)
		_, ok = g.GetTask(domain.NewInternedString("proj-b:test"))
		assert.True(t, ok)
	})

	t.Run("Workfile with tools", func(t *testing.T) {
		tmpDir := t.TempDir()

		workfileContent := `
version: "1"
tools:
  go: "go@1.23"
projects:
  - "project-a"
`
		workfilePath := filepath.Join(tmpDir, "same.work.yaml")
		err := os.WriteFile(workfilePath, []byte(workfileContent), 0o600)
		require.NoError(t, err)

		projectADir := filepath.Join(tmpDir, "project-a")
		err = os.Mkdir(projectADir, 0o750)
		require.NoError(t, err)
		samefileA := `
version: "1"
project: "proj-a"
tools:
  node: "nodejs@20"
tasks:
  build:
    cmd: ["go", "build"]
    tools: ["go", "node"]
`
		err = os.WriteFile(filepath.Join(projectADir, "same.yaml"), []byte(samefileA), 0o600)
		require.NoError(t, err)

		ctrl := gomock.NewController(t)
		loader := &config.Loader{Logger: mocks.NewMockLogger(ctrl)}
		g, err := loader.Load(tmpDir)
		require.NoError(t, err)

		task, ok := g.GetTask(domain.NewInternedString("proj-a:build"))
		require.True(t, ok)
		assert.Equal(t, map[string]string{
			"go":   "go@1.23",
			"node": "nodejs@20",
		}, task.Tools)
	})

	t.Run("Missing project field in samefile", func(t *testing.T) {
		tmpDir := t.TempDir()

		workfileContent := minimalWorkfileYAML
		workfilePath := filepath.Join(tmpDir, "same.work.yaml")
		err := os.WriteFile(workfilePath, []byte(workfileContent), 0o600)
		require.NoError(t, err)

		projectADir := filepath.Join(tmpDir, "project-a")
		err = os.Mkdir(projectADir, 0o750)
		require.NoError(t, err)
		samefileA := `
version: "1"
tasks:
  build:
    cmd: ["go", "build"]
`
		err = os.WriteFile(filepath.Join(projectADir, "same.yaml"), []byte(samefileA), 0o600)
		require.NoError(t, err)

		ctrl := gomock.NewController(t)
		loader := &config.Loader{Logger: mocks.NewMockLogger(ctrl)}
		_, err = loader.Load(tmpDir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing project name")
	})

	t.Run("Invalid project name", func(t *testing.T) {
		tmpDir := t.TempDir()

		workfileContent := minimalWorkfileYAML
		workfilePath := filepath.Join(tmpDir, "same.work.yaml")
		err := os.WriteFile(workfilePath, []byte(workfileContent), 0o600)
		require.NoError(t, err)

		projectADir := filepath.Join(tmpDir, "project-a")
		err = os.Mkdir(projectADir, 0o750)
		require.NoError(t, err)
		samefileA := `
version: "1"
project: "invalid:name"
tasks:
  build:
    cmd: ["go", "build"]
`
		err = os.WriteFile(filepath.Join(projectADir, "same.yaml"), []byte(samefileA), 0o600)
		require.NoError(t, err)

		ctrl := gomock.NewController(t)
		loader := &config.Loader{Logger: mocks.NewMockLogger(ctrl)}
		_, err = loader.Load(tmpDir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "project name can only contain")
	})

	t.Run("Duplicate project name", func(t *testing.T) {
		tmpDir := t.TempDir()

		workfileContent := `
version: "1"
projects:
  - "project-a"
  - "project-b"
`
		workfilePath := filepath.Join(tmpDir, "same.work.yaml")
		err := os.WriteFile(workfilePath, []byte(workfileContent), 0o600)
		require.NoError(t, err)

		// Create project-a
		projectADir := filepath.Join(tmpDir, "project-a")
		err = os.Mkdir(projectADir, 0o750)
		require.NoError(t, err)
		samefileA := `
version: "1"
project: "duplicate"
tasks:
  build:
    cmd: ["go", "build"]
`
		err = os.WriteFile(filepath.Join(projectADir, "same.yaml"), []byte(samefileA), 0o600)
		require.NoError(t, err)

		// Create project-b with same project name
		projectBDir := filepath.Join(tmpDir, "project-b")
		err = os.Mkdir(projectBDir, 0o750)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(projectBDir, "same.yaml"), []byte(samefileA), 0o600)
		require.NoError(t, err)

		ctrl := gomock.NewController(t)
		loader := &config.Loader{Logger: mocks.NewMockLogger(ctrl)}
		_, err = loader.Load(tmpDir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate project name")
	})

	t.Run("Root warning in workspace mode", func(t *testing.T) {
		tmpDir := t.TempDir()

		workfileContent := minimalWorkfileYAML
		workfilePath := filepath.Join(tmpDir, "same.work.yaml")
		err := os.WriteFile(workfilePath, []byte(workfileContent), 0o600)
		require.NoError(t, err)

		projectADir := filepath.Join(tmpDir, "project-a")
		err = os.Mkdir(projectADir, 0o750)
		require.NoError(t, err)
		samefileA := `
version: "1"
project: "proj-a"
root: "./custom-root"
tasks:
  build:
    cmd: ["go", "build"]
`
		err = os.WriteFile(filepath.Join(projectADir, "same.yaml"), []byte(samefileA), 0o600)
		require.NoError(t, err)

		ctrl := gomock.NewController(t)
		mockLogger := mocks.NewMockLogger(ctrl)
		mockLogger.EXPECT().Warn(gomock.Any()).Times(1)

		loader := &config.Loader{Logger: mockLogger}
		g, err := loader.Load(tmpDir)
		require.NoError(t, err)
		require.NotNil(t, g)
	})
}
