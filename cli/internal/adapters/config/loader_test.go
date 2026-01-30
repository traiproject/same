package config_test

import (
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/same/internal/adapters/config"
	"go.trai.ch/same/internal/core/domain"
	"go.trai.ch/same/internal/core/ports/mocks"
	"go.trai.ch/zerr"
	"go.uber.org/mock/gomock"
)

// Compile-time interface compliance checks.
var (
	_ config.FileSystem = (*config.OSFS)(nil)
	_ config.FileSystem = (*config.MapFSAdapter)(nil)
	_ config.FileSystem = (*MockFileSystem)(nil)
)

func TestNewLoader(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockLogger := mocks.NewMockLogger(ctrl)

	loader := config.NewLoader(mockLogger)
	require.NotNil(t, loader)
	require.Equal(t, mockLogger, loader.Logger)
	require.NotNil(t, loader.FS)
}

func TestNewLoaderWithFS(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockLogger := mocks.NewMockLogger(ctrl)

	testFS := config.NewMapFSAdapter("/workspace", fstest.MapFS{})
	loader := config.NewLoaderWithFS(mockLogger, testFS)
	require.NotNil(t, loader)
	require.Equal(t, mockLogger, loader.Logger)
	require.Equal(t, testFS, loader.FS)
}

func TestLoad_Unit_Success(t *testing.T) {
	tests := []struct {
		name         string
		files        fstest.MapFS
		cwd          string
		expectTasks  []string
		expectOrder  []string
		expectErr    bool
		expectErrMsg string
	}{
		{
			name: "standalone samefile with dependencies",
			files: fstest.MapFS{
				"same.yaml": &fstest.MapFile{
					Data: []byte(`
version: "1"
tasks:
  build:
    cmd: ["go", "build"]
    dependsOn: ["lint"]
  lint:
    cmd: ["golangci-lint", "run"]
`),
				},
			},
			cwd:         "/workspace",
			expectTasks: []string{"build", "lint"},
			expectOrder: []string{"lint", "build"},
		},
		{
			name: "standalone with environment",
			files: fstest.MapFS{
				"same.yaml": &fstest.MapFile{
					Data: []byte(`
version: "1"
tasks:
  build:
    cmd: ["go", "build"]
    environment:
      CGO_ENABLED: "0"
`),
				},
			},
			cwd:         "/workspace",
			expectTasks: []string{"build"},
		},
		{
			name: "standalone with tools",
			files: fstest.MapFS{
				"same.yaml": &fstest.MapFile{
					Data: []byte(`
version: "1"
tools:
  go: "go@1.23"
tasks:
  build:
    cmd: ["go", "build"]
    tools: ["go"]
`),
				},
			},
			cwd:         "/workspace",
			expectTasks: []string{"build"},
		},
		{
			name: "standalone with workingDir",
			files: fstest.MapFS{
				"same.yaml": &fstest.MapFile{
					Data: []byte(`
version: "1"
tasks:
  build:
    cmd: ["go", "build"]
    workingDir: "/custom/path"
`),
				},
			},
			cwd:         "/workspace",
			expectTasks: []string{"build"},
		},
		{
			name: "standalone with inputs and targets",
			files: fstest.MapFS{
				"same.yaml": &fstest.MapFile{
					Data: []byte(`
version: "1"
tasks:
  build:
    cmd: ["go", "build"]
    input: ["src/**/*.go"]
    target: ["bin/app"]
`),
				},
			},
			cwd:         "/workspace",
			expectTasks: []string{"build"},
		},
		{
			name: "workspace with projects",
			files: fstest.MapFS{
				"same.work.yaml": &fstest.MapFile{
					Data: []byte(`
version: "1"
projects:
  - "project-a"
  - "project-b"
`),
				},
				"project-a/same.yaml": &fstest.MapFile{
					Data: []byte(`
version: "1"
project: "proj-a"
tasks:
  build:
    cmd: ["go", "build"]
`),
				},
				"project-b/same.yaml": &fstest.MapFile{
					Data: []byte(`
version: "1"
project: "proj-b"
tasks:
  test:
    cmd: ["go", "test"]
`),
				},
			},
			cwd:         "/workspace",
			expectTasks: []string{"proj-a:build", "proj-b:test"},
		},
		{
			name: "workspace with glob pattern",
			files: fstest.MapFS{
				"same.work.yaml": &fstest.MapFile{
					Data: []byte(`
version: "1"
projects:
  - "packages/*"
`),
				},
				"packages/lib/same.yaml": &fstest.MapFile{
					Data: []byte(`
version: "1"
project: "lib"
tasks:
  build:
    cmd: ["go", "build"]
`),
				},
				"packages/app/same.yaml": &fstest.MapFile{
					Data: []byte(`
version: "1"
project: "app"
tasks:
  test:
    cmd: ["go", "test"]
`),
				},
			},
			cwd:         "/workspace",
			expectTasks: []string{"lib:build", "app:test"},
		},
		{
			name:         "config not found",
			files:        fstest.MapFS{},
			cwd:          "/workspace",
			expectErr:    true,
			expectErrMsg: "could not find samefile or workfile",
		},
		{
			name: "invalid yaml",
			files: fstest.MapFS{
				"same.yaml": &fstest.MapFile{
					Data: []byte(`invalid: yaml: content: [`),
				},
			},
			cwd:          "/workspace",
			expectErr:    true,
			expectErrMsg: "failed to parse config file",
		},
		{
			name: "missing dependency",
			files: fstest.MapFS{
				"same.yaml": &fstest.MapFile{
					Data: []byte(`
version: "1"
tasks:
  build:
    cmd: ["go", "build"]
    dependsOn: ["missing"]
`),
				},
			},
			cwd:          "/workspace",
			expectErr:    true,
			expectErrMsg: "missing dependency",
		},
		{
			name: "reserved task name 'all'",
			files: fstest.MapFS{
				"same.yaml": &fstest.MapFile{
					Data: []byte(`
version: "1"
tasks:
  all:
    cmd: ["echo", "hello"]
`),
				},
			},
			cwd:          "/workspace",
			expectErr:    true,
			expectErrMsg: "reserved",
		},
		{
			name: "invalid task name with colon",
			files: fstest.MapFS{
				"same.yaml": &fstest.MapFile{
					Data: []byte(`
version: "1"
tasks:
  "invalid:name":
    cmd: ["echo", "hello"]
`),
				},
			},
			cwd:          "/workspace",
			expectErr:    true,
			expectErrMsg: "invalid",
		},
		{
			name: "missing tool",
			files: fstest.MapFS{
				"same.yaml": &fstest.MapFile{
					Data: []byte(`
version: "1"
tasks:
  build:
    cmd: ["go", "build"]
    tools: ["missing-tool"]
`),
				},
			},
			cwd:          "/workspace",
			expectErr:    true,
			expectErrMsg: "tool not found",
		},
		{
			name: "invalid rebuild strategy",
			files: fstest.MapFS{
				"same.yaml": &fstest.MapFile{
					Data: []byte(`
version: "1"
tasks:
  build:
    cmd: ["go", "build"]
    rebuild: "invalid"
`),
				},
			},
			cwd:          "/workspace",
			expectErr:    true,
			expectErrMsg: "invalid rebuild strategy",
		},
		{
			name: "workspace missing project name",
			files: fstest.MapFS{
				"same.work.yaml": &fstest.MapFile{
					Data: []byte(`
version: "1"
projects:
  - "project-a"
`),
				},
				"project-a/same.yaml": &fstest.MapFile{
					Data: []byte(`
version: "1"
tasks:
  build:
    cmd: ["go", "build"]
`),
				},
			},
			cwd:          "/workspace",
			expectErr:    true,
			expectErrMsg: "missing project name",
		},
		{
			name: "workspace invalid project name",
			files: fstest.MapFS{
				"same.work.yaml": &fstest.MapFile{
					Data: []byte(`
version: "1"
projects:
  - "project-a"
`),
				},
				"project-a/same.yaml": &fstest.MapFile{
					Data: []byte(`
version: "1"
project: "invalid:name"
tasks:
  build:
    cmd: ["go", "build"]
`),
				},
			},
			cwd:          "/workspace",
			expectErr:    true,
			expectErrMsg: "project name can only contain",
		},
		{
			name: "workspace duplicate project name",
			files: fstest.MapFS{
				"same.work.yaml": &fstest.MapFile{
					Data: []byte(`
version: "1"
projects:
  - "project-a"
  - "project-b"
`),
				},
				"project-a/same.yaml": &fstest.MapFile{
					Data: []byte(`
version: "1"
project: "duplicate"
tasks:
  build:
    cmd: ["go", "build"]
`),
				},
				"project-b/same.yaml": &fstest.MapFile{
					Data: []byte(`
version: "1"
project: "duplicate"
tasks:
  test:
    cmd: ["go", "test"]
`),
				},
			},
			cwd:          "/workspace",
			expectErr:    true,
			expectErrMsg: "duplicate project name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockLogger := mocks.NewMockLogger(ctrl)

			testFS := config.NewMapFSAdapter("/workspace", tt.files)
			loader := config.NewLoaderWithFS(mockLogger, testFS)

			g, err := loader.Load(tt.cwd)

			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectErrMsg)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, g)

			// Validate graph
			err = g.Validate()
			require.NoError(t, err)

			// Collect tasks
			tasks := make(map[string]domain.Task)
			for task := range g.Walk() {
				tasks[task.Name.String()] = task
			}

			// Verify expected tasks exist
			for _, taskName := range tt.expectTasks {
				assert.Contains(t, tasks, taskName, "expected task %s to exist", taskName)
			}
			assert.Len(t, tasks, len(tt.expectTasks), "expected %d tasks", len(tt.expectTasks))

			// Verify order if specified
			if tt.expectOrder != nil {
				order := make([]string, 0, len(tt.expectOrder))
				for task := range g.Walk() {
					order = append(order, task.Name.String())
				}
				assert.Equal(t, tt.expectOrder, order)
			}
		})
	}
}

func TestLoad_Unit_RebuildStrategy(t *testing.T) {
	tests := []struct {
		name           string
		rebuildValue   string
		expectStrategy domain.RebuildStrategy
		expectErr      bool
	}{
		{
			name:           "default empty",
			rebuildValue:   "",
			expectStrategy: domain.RebuildOnChange,
		},
		{
			name:           "explicit on-change",
			rebuildValue:   "on-change",
			expectStrategy: domain.RebuildOnChange,
		},
		{
			name:           "always",
			rebuildValue:   "always",
			expectStrategy: domain.RebuildAlways,
		},
		{
			name:         "invalid",
			rebuildValue: "invalid",
			expectErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockLogger := mocks.NewMockLogger(ctrl)

			rebuildLine := ""
			if tt.rebuildValue != "" {
				rebuildLine = `    rebuild: "` + tt.rebuildValue + `"`
			}

			files := fstest.MapFS{
				"same.yaml": &fstest.MapFile{
					Data: []byte(`
version: "1"
tasks:
  build:
    cmd: ["go", "build"]
` + rebuildLine),
				},
			}

			testFS := config.NewMapFSAdapter("/workspace", files)
			loader := config.NewLoaderWithFS(mockLogger, testFS)

			g, err := loader.Load("/workspace")

			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "invalid rebuild strategy")
				return
			}

			require.NoError(t, err)

			task, ok := g.GetTask(domain.NewInternedString("build"))
			require.True(t, ok)
			assert.Equal(t, tt.expectStrategy, task.RebuildStrategy)
		})
	}
}

func TestLoad_Unit_Canonicalization(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockLogger := mocks.NewMockLogger(ctrl)

	files := fstest.MapFS{
		"same.yaml": &fstest.MapFile{
			Data: []byte(`
version: "1"
tasks:
  build:
    input: ["b", "a", "a", "c"]
    cmd: ["echo"]
    target: ["y", "x", "x", "z"]
`),
		},
	}

	testFS := config.NewMapFSAdapter("/workspace", files)
	loader := config.NewLoaderWithFS(mockLogger, testFS)

	g, err := loader.Load("/workspace")
	require.NoError(t, err)

	task, ok := g.GetTask(domain.NewInternedString("build"))
	require.True(t, ok)

	// Inputs should be sorted and deduplicated: a, b, c
	require.Len(t, task.Inputs, 3)
	assert.Equal(t, "a", task.Inputs[0].String())
	assert.Equal(t, "b", task.Inputs[1].String())
	assert.Equal(t, "c", task.Inputs[2].String())

	// Outputs should be sorted and deduplicated: x, y, z
	require.Len(t, task.Outputs, 3)
	assert.Equal(t, "x", task.Outputs[0].String())
	assert.Equal(t, "y", task.Outputs[1].String())
	assert.Equal(t, "z", task.Outputs[2].String())
}

func TestLoad_Unit_Workspace_NamespaceDependencies(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockLogger := mocks.NewMockLogger(ctrl)

	files := fstest.MapFS{
		"same.work.yaml": &fstest.MapFile{
			Data: []byte(`
version: "1"
projects:
  - "packages/*"
`),
		},
		"packages/lib/same.yaml": &fstest.MapFile{
			Data: []byte(`
version: "1"
project: "lib"
tasks:
  build:
    cmd: ["go", "build"]
`),
		},
		"packages/app/same.yaml": &fstest.MapFile{
			Data: []byte(`
version: "1"
project: "app"
tasks:
  test:
    cmd: ["go", "test"]
  build:
    cmd: ["go", "build"]
    dependsOn: ["test", "lib:build"]
`),
		},
	}

	testFS := config.NewMapFSAdapter("/workspace", files)
	loader := config.NewLoaderWithFS(mockLogger, testFS)

	g, err := loader.Load("/workspace")
	require.NoError(t, err)

	task, ok := g.GetTask(domain.NewInternedString("app:build"))
	require.True(t, ok)
	require.Len(t, task.Dependencies, 2)

	deps := make([]string, len(task.Dependencies))
	for i, dep := range task.Dependencies {
		deps[i] = dep.String()
	}

	assert.Contains(t, deps, "app:test", "expected local dependency to be namespaced")
	assert.Contains(t, deps, "lib:build", "expected cross-project dependency to remain unchanged")
}

func TestLoad_Unit_Workspace_ToolsInheritance(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockLogger := mocks.NewMockLogger(ctrl)

	files := fstest.MapFS{
		"same.work.yaml": &fstest.MapFile{
			Data: []byte(`
version: "1"
tools:
  go: "go@1.23"
  node: "nodejs@20"
projects:
  - "packages/*"
`),
		},
		"packages/myapp/same.yaml": &fstest.MapFile{
			Data: []byte(`
version: "1"
project: "myapp"
tasks:
  build:
    cmd: ["go", "build"]
    tools: ["go"]
  frontend:
    cmd: ["npm", "run", "build"]
    tools: ["node"]
`),
		},
	}

	testFS := config.NewMapFSAdapter("/workspace", files)
	loader := config.NewLoaderWithFS(mockLogger, testFS)

	g, err := loader.Load("/workspace")
	require.NoError(t, err)

	buildTask, ok := g.GetTask(domain.NewInternedString("myapp:build"))
	require.True(t, ok)
	assert.Equal(t, map[string]string{"go": "go@1.23"}, buildTask.Tools)

	frontendTask, ok := g.GetTask(domain.NewInternedString("myapp:frontend"))
	require.True(t, ok)
	assert.Equal(t, map[string]string{"node": "nodejs@20"}, frontendTask.Tools)
}

func TestLoad_Unit_Workspace_ToolsOverride(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockLogger := mocks.NewMockLogger(ctrl)

	files := fstest.MapFS{
		"same.work.yaml": &fstest.MapFile{
			Data: []byte(`
version: "1"
tools:
  go: "go@1.22"
projects:
  - "packages/*"
`),
		},
		"packages/myapp/same.yaml": &fstest.MapFile{
			Data: []byte(`
version: "1"
project: "myapp"
tools:
  go: "go@1.23"
tasks:
  build:
    cmd: ["go", "build"]
    tools: ["go"]
`),
		},
	}

	testFS := config.NewMapFSAdapter("/workspace", files)
	loader := config.NewLoaderWithFS(mockLogger, testFS)

	g, err := loader.Load("/workspace")
	require.NoError(t, err)

	task, ok := g.GetTask(domain.NewInternedString("myapp:build"))
	require.True(t, ok)
	assert.Equal(t, map[string]string{"go": "go@1.23"}, task.Tools)
}

func TestLoad_Unit_Workspace_ToolsMerge(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockLogger := mocks.NewMockLogger(ctrl)

	files := fstest.MapFS{
		"same.work.yaml": &fstest.MapFile{
			Data: []byte(`
version: "1"
tools:
  go: "go@1.23"
projects:
  - "packages/*"
`),
		},
		"packages/myapp/same.yaml": &fstest.MapFile{
			Data: []byte(`
version: "1"
project: "myapp"
tools:
  node: "nodejs@20"
tasks:
  fullstack:
    cmd: ["make", "build"]
    tools: ["go", "node"]
`),
		},
	}

	testFS := config.NewMapFSAdapter("/workspace", files)
	loader := config.NewLoaderWithFS(mockLogger, testFS)

	g, err := loader.Load("/workspace")
	require.NoError(t, err)

	task, ok := g.GetTask(domain.NewInternedString("myapp:fullstack"))
	require.True(t, ok)
	assert.Equal(t, map[string]string{
		"go":   "go@1.23",
		"node": "nodejs@20",
	}, task.Tools)
}

func TestLoad_Unit_ProjectFieldWarningInStandaloneMode(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockLogger := mocks.NewMockLogger(ctrl)

	// Expect the warning to be logged
	mockLogger.EXPECT().
		Warn(gomock.Eq("'project' defined in same.yaml has no effect in standalone mode")).
		Times(1)

	files := fstest.MapFS{
		"same.yaml": &fstest.MapFile{
			Data: []byte(`
version: "1"
project: "my-project"
tasks:
  build:
    cmd: ["go", "build"]
`),
		},
	}

	testFS := config.NewMapFSAdapter("/workspace", files)
	loader := config.NewLoaderWithFS(mockLogger, testFS)

	g, err := loader.Load("/workspace")
	require.NoError(t, err)
	require.NotNil(t, g)
}

func TestLoad_Unit_Workspace_RootWarning(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockLogger := mocks.NewMockLogger(ctrl)

	// Expect warning about root being ignored
	mockLogger.EXPECT().
		Warn(gomock.Any()).
		Do(func(msg string) {
			assert.Contains(t, msg, "'root' defined")
			assert.Contains(t, msg, "ignored in workspace mode")
		})

	files := fstest.MapFS{
		"same.work.yaml": &fstest.MapFile{
			Data: []byte(`
version: "1"
projects:
  - "project-a"
`),
		},
		"project-a/same.yaml": &fstest.MapFile{
			Data: []byte(`
version: "1"
project: "proj-a"
root: "./custom-root"
tasks:
  build:
    cmd: ["go", "build"]
`),
		},
	}

	testFS := config.NewMapFSAdapter("/workspace", files)
	loader := config.NewLoaderWithFS(mockLogger, testFS)

	g, err := loader.Load("/workspace")
	require.NoError(t, err)
	require.NotNil(t, g)
}

func TestLoad_Unit_Workspace_MissingSamefileWarning(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockLogger := mocks.NewMockLogger(ctrl)

	// Expect warning about missing same.yaml
	mockLogger.EXPECT().
		Warn(gomock.Any()).
		Do(func(msg string) {
			assert.Contains(t, msg, "missing")
			assert.Contains(t, msg, "skipping")
		})

	files := fstest.MapFS{
		"same.work.yaml": &fstest.MapFile{
			Data: []byte(`
version: "1"
projects:
  - "packages/*"
`),
		},
		"packages/empty-project": &fstest.MapFile{
			Mode: fs.ModeDir,
		},
	}

	testFS := config.NewMapFSAdapter("/workspace", files)
	loader := config.NewLoaderWithFS(mockLogger, testFS)

	g, err := loader.Load("/workspace")
	require.NoError(t, err)
	require.NotNil(t, g)
}

func TestLoad_Unit_ErrorMetadata(t *testing.T) {
	tests := []struct {
		name           string
		files          fstest.MapFS
		expectMetadata map[string]interface{}
	}{
		{
			name: "missing dependency metadata",
			files: fstest.MapFS{
				"same.yaml": &fstest.MapFile{
					Data: []byte(`
version: "1"
tasks:
  build:
    cmd: ["go", "build"]
    dependsOn: ["missing-dep"]
`),
				},
			},
			expectMetadata: map[string]interface{}{
				"missing_dependency": "missing-dep",
			},
		},
		{
			name: "reserved task name metadata",
			files: fstest.MapFS{
				"same.yaml": &fstest.MapFile{
					Data: []byte(`
version: "1"
tasks:
  all:
    cmd: ["echo", "hello"]
`),
				},
			},
			expectMetadata: map[string]interface{}{
				"task_name": "all",
			},
		},
		{
			name: "invalid task name metadata",
			files: fstest.MapFS{
				"same.yaml": &fstest.MapFile{
					Data: []byte(`
version: "1"
tasks:
  "bad:name":
    cmd: ["echo", "hello"]
`),
				},
			},
			expectMetadata: map[string]interface{}{
				"task_name":         "bad:name",
				"invalid_character": ":",
			},
		},
		{
			name: "missing tool metadata",
			files: fstest.MapFS{
				"same.yaml": &fstest.MapFile{
					Data: []byte(`
version: "1"
tasks:
  build:
    cmd: ["go", "build"]
    tools: ["missing-tool"]
`),
				},
			},
			expectMetadata: map[string]interface{}{
				"tool_alias": "missing-tool",
				"task":       "build",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockLogger := mocks.NewMockLogger(ctrl)

			testFS := config.NewMapFSAdapter("/workspace", tt.files)
			loader := config.NewLoaderWithFS(mockLogger, testFS)

			_, err := loader.Load("/workspace")
			require.Error(t, err)

			zErr, ok := err.(*zerr.Error)
			require.True(t, ok, "expected *zerr.Error, got %T", err)

			meta := zErr.Metadata()
			for key, expectedVal := range tt.expectMetadata {
				assert.Equal(t, expectedVal, meta[key], "metadata key %s", key)
			}
		})
	}
}

// MockFileSystem is a mock implementation of FileSystem for testing error paths.
type MockFileSystem struct {
	StatFunc     func(path string) (fs.FileInfo, error)
	ReadFileFunc func(path string) ([]byte, error)
	GlobFunc     func(pattern string) ([]string, error)
	IsDirFunc    func(path string) (bool, error)
}

func (m *MockFileSystem) Stat(path string) (fs.FileInfo, error) {
	if m.StatFunc != nil {
		return m.StatFunc(path)
	}
	return nil, errors.New("Stat not implemented")
}

func (m *MockFileSystem) ReadFile(path string) ([]byte, error) {
	if m.ReadFileFunc != nil {
		return m.ReadFileFunc(path)
	}
	return nil, errors.New("ReadFile not implemented")
}

func (m *MockFileSystem) Glob(pattern string) ([]string, error) {
	if m.GlobFunc != nil {
		return m.GlobFunc(pattern)
	}
	return nil, errors.New("Glob not implemented")
}

func (m *MockFileSystem) IsDir(path string) (bool, error) {
	if m.IsDirFunc != nil {
		return m.IsDirFunc(path)
	}
	return false, errors.New("IsDir not implemented")
}

func TestLoad_Unit_FilesystemErrors(t *testing.T) {
	tests := []struct {
		name         string
		setupMock    func() *MockFileSystem
		expectErr    bool
		expectErrMsg string
	}{
		{
			name: "Stat returns permission denied",
			setupMock: func() *MockFileSystem {
				return &MockFileSystem{
					StatFunc: func(_ string) (fs.FileInfo, error) {
						return nil, fs.ErrPermission
					},
				}
			},
			expectErr:    true,
			expectErrMsg: "could not find samefile or workfile",
		},
		{
			name: "ReadFile returns permission denied",
			setupMock: func() *MockFileSystem {
				return &MockFileSystem{
					StatFunc: func(_ string) (fs.FileInfo, error) {
						// First call finds the file
						return &mockFileInfo{isDir: false}, nil
					},
					ReadFileFunc: func(_ string) ([]byte, error) {
						return nil, fs.ErrPermission
					},
				}
			},
			expectErr:    true,
			expectErrMsg: "failed to read config file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockLogger := mocks.NewMockLogger(ctrl)

			mockFS := tt.setupMock()
			loader := config.NewLoaderWithFS(mockLogger, mockFS)

			_, err := loader.Load("/workspace")

			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectErrMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// mockFileInfo implements fs.FileInfo for testing.
type mockFileInfo struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
	isDir   bool
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return m.size }
func (m *mockFileInfo) Mode() fs.FileMode  { return m.mode }
func (m *mockFileInfo) ModTime() time.Time { return m.modTime }
func (m *mockFileInfo) IsDir() bool        { return m.isDir }
func (m *mockFileInfo) Sys() interface{}   { return nil }

// TestMapFSAdapter_PathBoundaryCheck verifies that paths outside the root are properly rejected.
func TestMapFSAdapter_PathBoundaryCheck(t *testing.T) {
	files := fstest.MapFS{
		"same.yaml": &fstest.MapFile{
			Data: []byte(`version: "1"`),
		},
		"subdir/file.txt": &fstest.MapFile{
			Data: []byte(`content`),
		},
	}

	t.Run("Normal root path validation", func(t *testing.T) {
		testFS := config.NewMapFSAdapter("/workspace", files)

		t.Run("Path within root succeeds", func(t *testing.T) {
			_, err := testFS.ReadFile("/workspace/same.yaml")
			require.NoError(t, err)
		})

		t.Run("Path in subdirectory within root succeeds", func(t *testing.T) {
			_, err := testFS.ReadFile("/workspace/subdir/file.txt")
			require.NoError(t, err)
		})

		t.Run("Path outside root fails - similar prefix", func(t *testing.T) {
			// This should fail because /workspace-other is NOT under /workspace
			// (tests the boundary check: /workspace-other shares a prefix but isn't a subdirectory)
			_, err := testFS.ReadFile("/workspace-other/same.yaml")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "file does not exist")
		})

		t.Run("Path outside root fails - different path", func(t *testing.T) {
			_, err := testFS.ReadFile("/other/same.yaml")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "file does not exist")
		})
	})

	t.Run("Root filesystem (/) - edge case", func(t *testing.T) {
		testFS := config.NewMapFSAdapter("/", files)

		t.Run("Any absolute path succeeds when root is /", func(t *testing.T) {
			_, err := testFS.ReadFile("/same.yaml")
			require.NoError(t, err)
		})

		t.Run("Nested path succeeds when root is /", func(t *testing.T) {
			_, err := testFS.ReadFile("/subdir/file.txt")
			require.NoError(t, err)
		})
	})
}
