package config_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/same/internal/adapters/config"
	"go.trai.ch/same/internal/core/domain"
	"go.trai.ch/same/internal/core/ports/mocks"
	"go.uber.org/mock/gomock"
)

func TestLoader_Load_WorkspaceLogic(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockLogger := mocks.NewMockLogger(ctrl)
	// Allow any logging, as we are testing logic, not strict log calls for now
	mockLogger.EXPECT().Warn(gomock.Any()).AnyTimes()

	loader := config.NewLoader(mockLogger)

	// Create temp workspace
	rootDir := t.TempDir()

	// 1. same.work.yaml
	workfileContent := `
version: "1"
root: .
tools:
  go: "1.21"
projects:
  - "pkg-*"
`
	createFile(t, rootDir, domain.WorkFileName, workfileContent)

	// 2. pkg-a (inherits tools)
	pkgADir := filepath.Join(rootDir, "pkg-a")
	err := os.Mkdir(pkgADir, domain.DirPerm)
	require.NoError(t, err)

	pkgASamefile := `
version: "1"
project: pkg-a
tasks:
  build:
    cmd: ["go", "build"]
    tools: ["go"]
`
	createFile(t, pkgADir, domain.SameFileName, pkgASamefile)

	// 3. pkg-b (overrides tools)
	pkgBDir := filepath.Join(rootDir, "pkg-b")
	err = os.Mkdir(pkgBDir, domain.DirPerm)
	require.NoError(t, err)

	pkgBSamefile := `
version: "1"
project: pkg-b
tools:
  go: "1.22"
tasks:
  test:
    cmd: ["go", "test"]
    tools: ["go"]
`
	createFile(t, pkgBDir, domain.SameFileName, pkgBSamefile)

	// Load
	g, err := loader.Load(rootDir)
	require.NoError(t, err)
	require.NotNil(t, g)

	// Verify pkg-a task (go 1.21)
	taskA, ok := g.GetTask(domain.NewInternedString("pkg-a:build"))
	require.True(t, ok, "task pkg-a:build not found")
	assert.Equal(t, "1.21", taskA.Tools["go"])

	// Verify pkg-b task (go 1.22)
	taskB, ok := g.GetTask(domain.NewInternedString("pkg-b:test"))
	require.True(t, ok, "task pkg-b:test not found")
	assert.Equal(t, "1.22", taskB.Tools["go"])

	// Verify namespacing
	_, ok = g.GetTask(domain.NewInternedString("build"))
	assert.False(t, ok, "task build should not exist (should be namespaced)")
}

func TestLoader_Load_PathResolution(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockLogger := mocks.NewMockLogger(ctrl)
	loader := config.NewLoader(mockLogger)

	rootDir := t.TempDir()

	// same.work.yaml
	createFile(t, rootDir, domain.WorkFileName, `
version: "1"
root: .
projects: ["pkg-c"]
tools: {}
`)

	// pkg-c with relative paths
	pkgCDir := filepath.Join(rootDir, "pkg-c")
	require.NoError(t, os.Mkdir(pkgCDir, domain.DirPerm))

	createFile(t, pkgCDir, domain.SameFileName, `
version: "1"
project: pkg-c
tasks:
  bundle:
    input: ["./src"]
    target: ["dist"]
`)

	g, err := loader.Load(rootDir)
	require.NoError(t, err)

	task, ok := g.GetTask(domain.NewInternedString("pkg-c:bundle"))
	require.True(t, ok)

	// Paths should be relative to workspace root (rootDir), not project dir (pkg-c)
	// So "pkg-c/src" and "pkg-c/dist"

	// Canonical path check involves interned strings
	// We can iterate or check existence
	require.Len(t, task.Inputs, 1)
	assert.Equal(t, "pkg-c/src", task.Inputs[0].String())

	require.Len(t, task.Outputs, 1)
	assert.Equal(t, "pkg-c/dist", task.Outputs[0].String())
}

func TestLoader_Load_ValidationErrors(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T, rootDir string)
		expectedErr error
		errContains string // Optional extra check for error text
	}{
		{
			name: "Duplicate Project Names",
			setup: func(t *testing.T, rootDir string) {
				t.Helper()
				createFile(t, rootDir, domain.WorkFileName, `
version: "1"
root: .
projects: ["dir1", "dir2"]
`)
				createPkg(t, rootDir, "dir1", "my-project")
				createPkg(t, rootDir, "dir2", "my-project") // Duplicate name.
			},
			expectedErr: domain.ErrDuplicateProjectName,
		},
		{
			name: "Missing Dependency (Standalone)",
			setup: func(t *testing.T, rootDir string) {
				t.Helper()
				// Standalone mode: only same.yaml in root.
				createFile(t, rootDir, domain.SameFileName, `
version: "1"
project: myproj
tasks:
  task1:
    dependsOn: ["non-existent"]
`)
			},
			expectedErr: domain.ErrMissingDependency,
		},
		{
			name: "Reserved Task Name 'all'",
			setup: func(t *testing.T, rootDir string) {
				t.Helper()
				createFile(t, rootDir, domain.WorkFileName, `
version: "1"
root: .
projects: ["dir1"]
`)
				dir1 := filepath.Join(rootDir, "dir1")
				err := os.Mkdir(dir1, domain.DirPerm)
				require.NoError(t, err)
				createFile(t, dir1, domain.SameFileName, `
version: "1"
project: dir1
tasks:
  all:
    cmd: ["echo"]
`)
			},
			expectedErr: domain.ErrReservedTaskName,
		},
		{
			name: "Invalid YAML Syntax",
			setup: func(t *testing.T, rootDir string) {
				t.Helper()
				createFile(t, rootDir, domain.WorkFileName, `
version: "1"
root: .
projects: ["dir1"]
`)
				dir1 := filepath.Join(rootDir, "dir1")
				err := os.Mkdir(dir1, domain.DirPerm)
				require.NoError(t, err)
				createFile(t, dir1, domain.SameFileName, `
version: "1"
project: dir1
tasks: [ INVALID YAML ]
`)
			},
			expectedErr: nil, // Error is wrapped, check string below.
			errContains: "failed to parse project config",
		},
		{
			name: "Missing Tool",
			setup: func(t *testing.T, rootDir string) {
				t.Helper()
				createFile(t, rootDir, domain.WorkFileName, `
version: "1"
root: .
projects: ["dir1"]
tools: {}
`)
				dir1 := filepath.Join(rootDir, "dir1")
				err := os.Mkdir(dir1, domain.DirPerm)
				require.NoError(t, err)
				createFile(t, dir1, domain.SameFileName, `
version: "1"
project: dir1
tasks:
  build:
    tools: ["python"] # Not defined.
`)
			},
			expectedErr: domain.ErrMissingTool,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockLogger := mocks.NewMockLogger(ctrl)
			// Some errors might log before returning, allow it
			mockLogger.EXPECT().Warn(gomock.Any()).AnyTimes()

			loader := config.NewLoader(mockLogger)
			rootDir := t.TempDir()

			tt.setup(t, rootDir)

			g, err := loader.Load(rootDir)
			// assert.ErrorIs might fail if there's an import mismatch or if zerr wraps differently than expected by testify.
			// Falling back to string check as requested ("assert specific error messages").
			switch {
			case tt.expectedErr != nil:
				require.Error(t, err)
				require.ErrorContains(t, err, tt.expectedErr.Error())
			case tt.errContains != "":
				require.Error(t, err)
				require.ErrorContains(t, err, tt.errContains)
			default:
				require.NoError(t, err)
			}

			assert.Nil(t, g)
		})
	}
}

// Helpers.

func createFile(t *testing.T, dir, name, content string) {
	t.Helper()
	err := os.WriteFile(filepath.Join(dir, name), []byte(content), domain.PrivateFilePerm)
	require.NoError(t, err)
}

func createPkg(t *testing.T, rootDir, dirName, projectName string) {
	t.Helper()
	dir := filepath.Join(rootDir, dirName)
	err := os.MkdirAll(dir, domain.DirPerm)
	require.NoError(t, err)

	content := fmt.Sprintf(`
version: "1"
project: "%s"
tasks: {}
`, projectName)
	createFile(t, dir, domain.SameFileName, content)
}
