package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/bob/internal/adapters/config"
	"go.trai.ch/bob/internal/core/ports/mocks"
	"go.trai.ch/zerr"
	"go.uber.org/mock/gomock"
)

const testWorkspaceConfig = `
version: "1"
projects:
  - "packages/*"
`

// TestLoad_DuplicateProjectName tests duplicate project name detection in workspaces.
func TestLoad_DuplicateProjectName(t *testing.T) {
	tmpDir := t.TempDir()

	// Create workspace config
	workfilePath := filepath.Join(tmpDir, "bob.work.yaml")
	err := os.WriteFile(workfilePath, []byte(testWorkspaceConfig), 0o600)
	require.NoError(t, err)

	// Create packages directory
	packagesDir := filepath.Join(tmpDir, "packages")
	err = os.MkdirAll(packagesDir, 0o750)
	require.NoError(t, err)

	// Create first project with name "myapp"
	project1Dir := filepath.Join(packagesDir, "project1")
	err = os.MkdirAll(project1Dir, 0o750)
	require.NoError(t, err)
	bobfile1Content := `
version: "1"
project: "myapp"
tasks:
  build:
    cmd: ["echo", "building project1"]
`
	bobfile1Path := filepath.Join(project1Dir, "bob.yaml")
	err = os.WriteFile(bobfile1Path, []byte(bobfile1Content), 0o600)
	require.NoError(t, err)

	// Create second project with the same name "myapp"
	project2Dir := filepath.Join(packagesDir, "project2")
	err = os.MkdirAll(project2Dir, 0o750)
	require.NoError(t, err)
	bobfile2Content := `
version: "1"
project: "myapp"
tasks:
  test:
    cmd: ["echo", "testing project2"]
`
	bobfile2Path := filepath.Join(project2Dir, "bob.yaml")
	err = os.WriteFile(bobfile2Path, []byte(bobfile2Content), 0o600)
	require.NoError(t, err)

	// Load the config
	ctrl := gomock.NewController(t)
	mockLogger := mocks.NewMockLogger(ctrl)
	loader := &config.Loader{Logger: mockLogger}

	_, err = loader.Load(tmpDir)
	require.Error(t, err, "expected error for duplicate project name")

	// Verify it's the correct error type
	zErr, ok := err.(*zerr.Error)
	require.True(t, ok, "expected *zerr.Error, got %T: %v", err, err)

	// Check error is domain.ErrDuplicateProjectName
	assert.Contains(t, err.Error(), "duplicate project name")

	// Verify error metadata
	meta := zErr.Metadata()
	assert.Equal(t, "myapp", meta["project_name"], "expected project_name metadata")
	assert.Contains(t, meta, "first_occurrence", "expected first_occurrence metadata")
	assert.Contains(t, meta, "duplicate_at", "expected duplicate_at metadata")

	// The order of processing depends on sorting, so we need to check both possibilities
	first := meta["first_occurrence"].(string)
	duplicate := meta["duplicate_at"].(string)

	assert.True(t,
		(first == "packages/project1" && duplicate == "packages/project2") ||
			(first == "packages/project2" && duplicate == "packages/project1"),
		"expected first_occurrence and duplicate_at to be packages/project1 and packages/project2 in some order")
}

// TestLoad_Workspace_NamespaceDependencies tests the namespaceDependencies function
// by creating a workspace with tasks that have both local and cross-project dependencies.
func TestLoad_Workspace_NamespaceDependencies(t *testing.T) {
	tmpDir := t.TempDir()

	// Create workspace config
	workfilePath := filepath.Join(tmpDir, "bob.work.yaml")
	err := os.WriteFile(workfilePath, []byte(testWorkspaceConfig), 0o600)
	require.NoError(t, err)

	// Create packages directory
	packagesDir := filepath.Join(tmpDir, "packages")
	err = os.MkdirAll(packagesDir, 0o750)
	require.NoError(t, err)

	// Create first project
	project1Dir := filepath.Join(packagesDir, "lib")
	err = os.MkdirAll(project1Dir, 0o750)
	require.NoError(t, err)

	bobfile1Content := `
version: "1"
project: "lib"
tasks:
  build:
    cmd: ["go", "build"]
`
	bobfile1Path := filepath.Join(project1Dir, "bob.yaml")
	err = os.WriteFile(bobfile1Path, []byte(bobfile1Content), 0o600)
	require.NoError(t, err)

	// Create second project with dependencies (local and cross-project)
	project2Dir := filepath.Join(packagesDir, "app")
	err = os.MkdirAll(project2Dir, 0o750)
	require.NoError(t, err)

	bobfile2Content := `
version: "1"
project: "app"
tasks:
  test:
    cmd: ["go", "test"]
  build:
    cmd: ["go", "build"]
    dependsOn: ["test", "lib:build"]
`
	bobfile2Path := filepath.Join(project2Dir, "bob.yaml")
	err = os.WriteFile(bobfile2Path, []byte(bobfile2Content), 0o600)
	require.NoError(t, err)

	// Load the config
	ctrl := gomock.NewController(t)
	mockLogger := mocks.NewMockLogger(ctrl)
	loader := &config.Loader{Logger: mockLogger}

	g, err := loader.Load(tmpDir)
	require.NoError(t, err)
	require.NotNil(t, g)

	// Verify the task dependencies were namespaced correctly
	for task := range g.Walk() {
		if task.Name.String() != "app:build" {
			continue
		}
		require.Len(t, task.Dependencies, 2)

		// Dependencies should be sorted and deduplicated
		deps := make([]string, len(task.Dependencies))
		for i, dep := range task.Dependencies {
			deps[i] = dep.String()
		}

		// Should contain both local (namespaced) and cross-project dependencies
		assert.Contains(t, deps, "app:test", "expected local dependency to be namespaced")
		assert.Contains(t, deps, "lib:build", "expected cross-project dependency to remain unchanged")
	}
}

// TestLoad_Workspace_InvalidProjectName tests the validateBobfile function
// by creating a workspace with an invalid project name.
func TestLoad_Workspace_InvalidProjectName(t *testing.T) {
	tests := []struct {
		name        string
		projectName string
		shouldError bool
	}{
		{
			name:        "Valid project name",
			projectName: "valid-project_123",
			shouldError: false,
		},
		{
			name:        "Invalid project name with space",
			projectName: "invalid project",
			shouldError: true,
		},
		{
			name:        "Invalid project name with special char",
			projectName: "invalid@project",
			shouldError: true,
		},
		{
			name:        "Invalid project name with dot",
			projectName: "invalid.project",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Create workspace config
			workfilePath := filepath.Join(tmpDir, "bob.work.yaml")
			err := os.WriteFile(workfilePath, []byte(testWorkspaceConfig), 0o600)
			require.NoError(t, err)

			// Create packages directory
			packagesDir := filepath.Join(tmpDir, "packages")
			err = os.MkdirAll(packagesDir, 0o750)
			require.NoError(t, err)

			// Create project
			projectDir := filepath.Join(packagesDir, "project1")
			err = os.MkdirAll(projectDir, 0o750)
			require.NoError(t, err)

			bobfileContent := `
version: "1"
project: "` + tt.projectName + `"
tasks:
  build:
    cmd: ["echo", "building"]
`
			bobfilePath := filepath.Join(projectDir, "bob.yaml")
			err = os.WriteFile(bobfilePath, []byte(bobfileContent), 0o600)
			require.NoError(t, err)

			// Load the config
			ctrl := gomock.NewController(t)
			mockLogger := mocks.NewMockLogger(ctrl)
			loader := &config.Loader{Logger: mockLogger}

			_, err = loader.Load(tmpDir)

			if tt.shouldError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "project name can only contain")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestLoad_Workspace_MissingProjectName tests the validateBobfile function
// when the project name is missing.
func TestLoad_Workspace_MissingProjectName(t *testing.T) {
	tmpDir := t.TempDir()

	// Create workspace config
	workfilePath := filepath.Join(tmpDir, "bob.work.yaml")
	err := os.WriteFile(workfilePath, []byte(testWorkspaceConfig), 0o600)
	require.NoError(t, err)

	// Create packages directory
	packagesDir := filepath.Join(tmpDir, "packages")
	err = os.MkdirAll(packagesDir, 0o750)
	require.NoError(t, err)

	// Create project without project name
	projectDir := filepath.Join(packagesDir, "project1")
	err = os.MkdirAll(projectDir, 0o750)
	require.NoError(t, err)

	bobfileContent := `
version: "1"
tasks:
  build:
    cmd: ["echo", "building"]
`
	bobfilePath := filepath.Join(projectDir, "bob.yaml")
	err = os.WriteFile(bobfilePath, []byte(bobfileContent), 0o600)
	require.NoError(t, err)

	// Load the config
	ctrl := gomock.NewController(t)
	mockLogger := mocks.NewMockLogger(ctrl)
	loader := &config.Loader{Logger: mockLogger}

	_, err = loader.Load(tmpDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing project name")
}

// TestLoad_Workspace_BobfileReadError tests error handling when bob.yaml cannot be read.
func TestLoad_Workspace_BobfileReadError(t *testing.T) {
	tmpDir := t.TempDir()

	// Create workspace config
	workfilePath := filepath.Join(tmpDir, "bob.work.yaml")
	err := os.WriteFile(workfilePath, []byte(testWorkspaceConfig), 0o600)
	require.NoError(t, err)

	// Create packages directory
	packagesDir := filepath.Join(tmpDir, "packages")
	err = os.MkdirAll(packagesDir, 0o750)
	require.NoError(t, err)

	// Create project directory
	projectDir := filepath.Join(packagesDir, "project1")
	err = os.MkdirAll(projectDir, 0o750)
	require.NoError(t, err)

	// Create bob.yaml with no read permissions
	bobfilePath := filepath.Join(projectDir, "bob.yaml")
	err = os.WriteFile(bobfilePath, []byte("test"), 0o000)
	require.NoError(t, err)

	// Load the config
	ctrl := gomock.NewController(t)
	mockLogger := mocks.NewMockLogger(ctrl)
	loader := &config.Loader{Logger: mockLogger}

	_, err = loader.Load(tmpDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config file")
}

// TestLoad_Workspace_BobfileParseError tests error handling when bob.yaml has invalid YAML.
func TestLoad_Workspace_BobfileParseError(t *testing.T) {
	tmpDir := t.TempDir()

	// Create workspace config
	workfilePath := filepath.Join(tmpDir, "bob.work.yaml")
	err := os.WriteFile(workfilePath, []byte(testWorkspaceConfig), 0o600)
	require.NoError(t, err)

	// Create packages directory
	packagesDir := filepath.Join(tmpDir, "packages")
	err = os.MkdirAll(packagesDir, 0o750)
	require.NoError(t, err)

	// Create project directory
	projectDir := filepath.Join(packagesDir, "project1")
	err = os.MkdirAll(projectDir, 0o750)
	require.NoError(t, err)

	// Create bob.yaml with invalid YAML
	bobfilePath := filepath.Join(projectDir, "bob.yaml")
	invalidYaml := `
version: "1"
project: "test"
tasks:
  build: [invalid yaml structure
`
	err = os.WriteFile(bobfilePath, []byte(invalidYaml), 0o600)
	require.NoError(t, err)

	// Load the config
	ctrl := gomock.NewController(t)
	mockLogger := mocks.NewMockLogger(ctrl)
	loader := &config.Loader{Logger: mockLogger}

	_, err = loader.Load(tmpDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse project config")
}

// TestLoad_Workspace_RootWarning tests that the loader warns when root is defined
// in a workspace project's bob.yaml (which is ignored).
func TestLoad_Workspace_RootWarning(t *testing.T) {
	tmpDir := t.TempDir()

	// Create workspace config
	workfilePath := filepath.Join(tmpDir, "bob.work.yaml")
	err := os.WriteFile(workfilePath, []byte(testWorkspaceConfig), 0o600)
	require.NoError(t, err)

	// Create packages directory
	packagesDir := filepath.Join(tmpDir, "packages")
	err = os.MkdirAll(packagesDir, 0o750)
	require.NoError(t, err)

	// Create project with root defined (should be ignored and warned)
	projectDir := filepath.Join(packagesDir, "project1")
	err = os.MkdirAll(projectDir, 0o750)
	require.NoError(t, err)

	bobfileContent := `
version: "1"
project: "myproject"
root: "./custom-root"
tasks:
  build:
    cmd: ["echo", "building"]
`
	bobfilePath := filepath.Join(projectDir, "bob.yaml")
	err = os.WriteFile(bobfilePath, []byte(bobfileContent), 0o600)
	require.NoError(t, err)

	// Load the config with mock logger to capture warnings
	ctrl := gomock.NewController(t)
	mockLogger := mocks.NewMockLogger(ctrl)

	// Expect warning about root being ignored
	mockLogger.EXPECT().
		Warn(gomock.Any()).
		Do(func(msg string) {
			assert.Contains(t, msg, "'root' defined")
			assert.Contains(t, msg, "ignored in workspace mode")
		})

	loader := &config.Loader{Logger: mockLogger}

	g, err := loader.Load(tmpDir)
	require.NoError(t, err)
	require.NotNil(t, g)
}

// TestLoad_Workspace_MissingBobfileWarning tests that the loader warns when
// a matched directory doesn't contain a bob.yaml file.
func TestLoad_Workspace_MissingBobfileWarning(t *testing.T) {
	tmpDir := t.TempDir()

	// Create workspace config
	workfilePath := filepath.Join(tmpDir, "bob.work.yaml")
	err := os.WriteFile(workfilePath, []byte(testWorkspaceConfig), 0o600)
	require.NoError(t, err)

	// Create packages directory
	packagesDir := filepath.Join(tmpDir, "packages")
	err = os.MkdirAll(packagesDir, 0o750)
	require.NoError(t, err)

	// Create project directory WITHOUT bob.yaml
	projectDir := filepath.Join(packagesDir, "empty-project")
	err = os.MkdirAll(projectDir, 0o750)
	require.NoError(t, err)

	// Load the config with mock logger to capture warnings
	ctrl := gomock.NewController(t)
	mockLogger := mocks.NewMockLogger(ctrl)

	// Expect warning about missing bob.yaml
	mockLogger.EXPECT().
		Warn(gomock.Any()).
		Do(func(msg string) {
			assert.Contains(t, msg, "missing")
			assert.Contains(t, msg, "skipping")
		})

	loader := &config.Loader{Logger: mockLogger}

	g, err := loader.Load(tmpDir)
	require.NoError(t, err)
	require.NotNil(t, g)
}
