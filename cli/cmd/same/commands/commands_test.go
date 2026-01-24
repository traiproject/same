package commands_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"go.trai.ch/same/cmd/same/commands"
	"go.trai.ch/same/internal/app"
	"go.trai.ch/same/internal/core/domain"
	"go.trai.ch/same/internal/core/ports/mocks"
	"go.uber.org/mock/gomock"
)

func TestRun_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockLoader := mocks.NewMockConfigLoader(ctrl)
	mockExecutor := mocks.NewMockExecutor(ctrl)
	mockStore := mocks.NewMockBuildInfoStore(ctrl)
	mockHasher := mocks.NewMockHasher(ctrl)
	mockResolver := mocks.NewMockInputResolver(ctrl)
	mockEnvFactory := mocks.NewMockEnvironmentFactory(ctrl)

	// Create a graph with one task named "build"
	g := domain.NewGraph()
	buildTask := &domain.Task{Name: domain.NewInternedString("build"), WorkingDir: domain.NewInternedString("Root")}
	_ = g.AddTask(buildTask)

	// Setup app
	mockLogger := mocks.NewMockLogger(ctrl)
	a := app.New(mockLoader, mockExecutor, mockLogger, mockStore, mockHasher, mockResolver, mockEnvFactory).
		WithTeaOptions(tea.WithInput(nil), tea.WithOutput(io.Discard))

	// Initialize CLI
	cli := commands.New(a)

	// Setup strict expectations in the correct sequence
	// 1. Loader.Load is called first
	mockLoader.EXPECT().Load(".").Return(g, nil).Times(1)

	mockResolver.EXPECT().ResolveInputs(gomock.Any(), gomock.Any()).Return([]string{}, nil).Times(1)
	// 2. Hasher.ComputeInputHash is called once to compute input hash
	mockHasher.EXPECT().ComputeInputHash(gomock.Any(), gomock.Any(), gomock.Any()).Return("hash123", nil).Times(1)

	// 3. Store.Get is called once to check for cached build info (simulate cache miss by returning nil)
	mockStore.EXPECT().Get("build").Return(nil, nil).Times(1)

	// 4. Executor.Execute is called once to run the task (since it's a cache miss)
	mockExecutor.EXPECT().Execute(
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
	).Return(nil).Times(1)

	// 5. Store.Put is called once to save the new build result
	mockStore.EXPECT().Put(gomock.Any()).Return(nil).Times(1)

	// Set command args
	cli.SetArgs([]string{"run", "build"})

	// Execute
	err := cli.Execute(context.Background())
	// Assert
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestRun_NoTargets(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockLoader := mocks.NewMockConfigLoader(ctrl)
	mockExecutor := mocks.NewMockExecutor(ctrl)
	mockStore := mocks.NewMockBuildInfoStore(ctrl)
	mockHasher := mocks.NewMockHasher(ctrl)
	mockResolver := mocks.NewMockInputResolver(ctrl)
	mockEnvFactory := mocks.NewMockEnvironmentFactory(ctrl)

	// Setup app
	mockLogger := mocks.NewMockLogger(ctrl)
	a := app.New(mockLoader, mockExecutor, mockLogger, mockStore, mockHasher, mockResolver, mockEnvFactory).
		WithTeaOptions(tea.WithInput(nil), tea.WithOutput(io.Discard))

	// Initialize CLI
	cli := commands.New(a)

	// Set command args (no targets)
	cli.SetArgs([]string{"run"})

	// Execute
	err := cli.Execute(context.Background())
	// With the updated implementation, no error should be returned
	// when no targets are provided (just displays help)
	if err != nil {
		t.Errorf("Expected no error for no targets, got: %v", err)
	}
}

func TestRoot_Help(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockLoader := mocks.NewMockConfigLoader(ctrl)
	mockExecutor := mocks.NewMockExecutor(ctrl)
	mockStore := mocks.NewMockBuildInfoStore(ctrl)
	mockHasher := mocks.NewMockHasher(ctrl)
	mockResolver := mocks.NewMockInputResolver(ctrl)
	mockEnvFactory := mocks.NewMockEnvironmentFactory(ctrl)

	// Setup app
	mockLogger := mocks.NewMockLogger(ctrl)
	a := app.New(mockLoader, mockExecutor, mockLogger, mockStore, mockHasher, mockResolver, mockEnvFactory).
		WithTeaOptions(tea.WithInput(nil), tea.WithOutput(io.Discard))

	// Initialize CLI
	cli := commands.New(a)

	// Set command args to help
	cli.SetArgs([]string{"--help"})

	// Execute
	err := cli.Execute(context.Background())
	// Assert no error (Cobra handles help automatically)
	if err != nil {
		t.Errorf("Expected no error for help, got: %v", err)
	}
}

func TestRoot_Version(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockLoader := mocks.NewMockConfigLoader(ctrl)
	mockExecutor := mocks.NewMockExecutor(ctrl)
	mockStore := mocks.NewMockBuildInfoStore(ctrl)
	mockHasher := mocks.NewMockHasher(ctrl)
	mockResolver := mocks.NewMockInputResolver(ctrl)
	mockEnvFactory := mocks.NewMockEnvironmentFactory(ctrl)

	// Setup app
	mockLogger := mocks.NewMockLogger(ctrl)
	a := app.New(mockLoader, mockExecutor, mockLogger, mockStore, mockHasher, mockResolver, mockEnvFactory).
		WithTeaOptions(tea.WithInput(nil), tea.WithOutput(io.Discard))

	// Initialize CLI
	cli := commands.New(a)

	// Set command args to version
	cli.SetArgs([]string{"--version"})

	// Execute
	err := cli.Execute(context.Background())
	// Assert no error (Cobra handles version automatically)
	if err != nil {
		t.Errorf("Expected no error for version, got: %v", err)
	}
}

func TestVersionCmd(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockLoader := mocks.NewMockConfigLoader(ctrl)
	mockExecutor := mocks.NewMockExecutor(ctrl)
	mockStore := mocks.NewMockBuildInfoStore(ctrl)
	mockHasher := mocks.NewMockHasher(ctrl)
	mockResolver := mocks.NewMockInputResolver(ctrl)
	mockEnvFactory := mocks.NewMockEnvironmentFactory(ctrl)

	// Setup app
	mockLogger := mocks.NewMockLogger(ctrl)
	a := app.New(mockLoader, mockExecutor, mockLogger, mockStore, mockHasher, mockResolver, mockEnvFactory).
		WithTeaOptions(tea.WithInput(nil), tea.WithOutput(io.Discard))

	// Initialize CLI
	cli := commands.New(a)

	// Set command args to version subcommand
	cli.SetArgs([]string{"version"})

	// Execute
	err := cli.Execute(context.Background())
	// Assert no error
	if err != nil {
		t.Errorf("Expected no error for version command, got: %v", err)
	}
}

// setupCleanTest creates a test CLI with mocked dependencies for clean command tests.
func setupCleanTest(t *testing.T) (*commands.CLI, *mocks.MockLogger) {
	t.Helper()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}
	defer func() {
		if errChdir := os.Chdir(cwd); errChdir != nil {
			t.Fatalf("Failed to restore working directory: %v", errChdir)
		}
	}()

	tmpDir := t.TempDir()
	if errChdir := os.Chdir(tmpDir); errChdir != nil {
		t.Fatalf("Failed to change into temp directory: %v", errChdir)
	}

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockLoader := mocks.NewMockConfigLoader(ctrl)
	mockExecutor := mocks.NewMockExecutor(ctrl)
	mockStore := mocks.NewMockBuildInfoStore(ctrl)
	mockHasher := mocks.NewMockHasher(ctrl)
	mockResolver := mocks.NewMockInputResolver(ctrl)
	mockEnvFactory := mocks.NewMockEnvironmentFactory(ctrl)
	mockLogger := mocks.NewMockLogger(ctrl)

	a := app.New(mockLoader, mockExecutor, mockLogger, mockStore, mockHasher, mockResolver, mockEnvFactory).
		WithTeaOptions(tea.WithInput(nil), tea.WithOutput(io.Discard))

	return commands.New(a), mockLogger
}

func createDirWithMarker(t *testing.T, dirPath string) {
	t.Helper()
	if err := os.MkdirAll(dirPath, domain.DirPerm); err != nil {
		t.Fatalf("Failed to create directory %s: %v", dirPath, err)
	}
	markerFile := filepath.Join(dirPath, "marker.txt")
	if err := os.WriteFile(markerFile, []byte("test"), domain.FilePerm); err != nil {
		t.Fatalf("Failed to create marker file in %s: %v", dirPath, err)
	}
}

func TestCleanCmd_Default(t *testing.T) {
	cli, mockLogger := setupCleanTest(t)
	mockLogger.EXPECT().Info(gomock.Any()).AnyTimes()

	storePath := filepath.Join(domain.DefaultSamePath(), domain.StoreDirName)
	if err := os.MkdirAll(storePath, domain.DirPerm); err != nil {
		t.Fatalf("Failed to create store directory: %v", err)
	}
	markerFile := filepath.Join(storePath, "marker.txt")
	if err := os.WriteFile(markerFile, []byte("test"), domain.FilePerm); err != nil {
		t.Fatalf("Failed to create marker file: %v", err)
	}

	cli.SetArgs([]string{"clean"})
	err := cli.Execute(context.Background())
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if _, err := os.Stat(storePath); !os.IsNotExist(err) {
		t.Errorf("Expected store directory to be removed, but it still exists")
	}
}

func TestCleanCmd_Tools(t *testing.T) {
	cli, mockLogger := setupCleanTest(t)
	mockLogger.EXPECT().Info(gomock.Any()).AnyTimes()

	nixHubPath := domain.DefaultNixHubCachePath()
	if err := os.MkdirAll(nixHubPath, domain.DirPerm); err != nil {
		t.Fatalf("Failed to create nixhub cache directory: %v", err)
	}
	nixMarker := filepath.Join(nixHubPath, "marker.txt")
	if err := os.WriteFile(nixMarker, []byte("test"), domain.FilePerm); err != nil {
		t.Fatalf("Failed to create marker file: %v", err)
	}

	envPath := domain.DefaultEnvCachePath()
	if err := os.MkdirAll(envPath, domain.DirPerm); err != nil {
		t.Fatalf("Failed to create env cache directory: %v", err)
	}
	envMarker := filepath.Join(envPath, "marker.txt")
	if err := os.WriteFile(envMarker, []byte("test"), domain.FilePerm); err != nil {
		t.Fatalf("Failed to create marker file: %v", err)
	}

	cli.SetArgs([]string{"clean", "--tools"})
	err := cli.Execute(context.Background())
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if _, err := os.Stat(nixHubPath); !os.IsNotExist(err) {
		t.Errorf("Expected nixhub cache directory to be removed, but it still exists")
	}
	if _, err := os.Stat(envPath); !os.IsNotExist(err) {
		t.Errorf("Expected environment cache directory to be removed, but it still exists")
	}
}

func TestCleanCmd_All(t *testing.T) {
	cli, mockLogger := setupCleanTest(t)
	mockLogger.EXPECT().Info(gomock.Any()).AnyTimes()

	storePath := filepath.Join(domain.DefaultSamePath(), domain.StoreDirName)
	createDirWithMarker(t, storePath)

	nixHubPath := domain.DefaultNixHubCachePath()
	createDirWithMarker(t, nixHubPath)

	envPath := domain.DefaultEnvCachePath()
	createDirWithMarker(t, envPath)

	cli.SetArgs([]string{"clean", "--all"})
	err := cli.Execute(context.Background())
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if _, err := os.Stat(storePath); !os.IsNotExist(err) {
		t.Errorf("Expected store directory to be removed, but it still exists")
	}
	if _, err := os.Stat(nixHubPath); !os.IsNotExist(err) {
		t.Errorf("Expected nixhub cache directory to be removed, but it still exists")
	}
	if _, err := os.Stat(envPath); !os.IsNotExist(err) {
		t.Errorf("Expected environment cache directory to be removed, but it still exists")
	}
}

func TestRun_OutputModeFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "output-mode flag",
			args: []string{"run", "--output-mode=linear", "build"},
		},
		{
			name: "ci flag",
			args: []string{"run", "--ci", "build"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockLoader := mocks.NewMockConfigLoader(ctrl)
			mockExecutor := mocks.NewMockExecutor(ctrl)
			mockStore := mocks.NewMockBuildInfoStore(ctrl)
			mockHasher := mocks.NewMockHasher(ctrl)
			mockResolver := mocks.NewMockInputResolver(ctrl)
			mockEnvFactory := mocks.NewMockEnvironmentFactory(ctrl)

			g := domain.NewGraph()
			g.SetRoot(".")
			buildTask := &domain.Task{Name: domain.NewInternedString("build"), WorkingDir: domain.NewInternedString("Root")}
			_ = g.AddTask(buildTask)

			mockLogger := mocks.NewMockLogger(ctrl)
			a := app.New(mockLoader, mockExecutor, mockLogger, mockStore, mockHasher, mockResolver, mockEnvFactory).
				WithTeaOptions(tea.WithInput(nil), tea.WithOutput(io.Discard))

			cli := commands.New(a)

			mockLoader.EXPECT().Load(".").Return(g, nil).Times(1)
			mockResolver.EXPECT().ResolveInputs(gomock.Any(), ".").Return([]string{}, nil).Times(1)
			mockHasher.EXPECT().ComputeInputHash(gomock.Any(), gomock.Any(), gomock.Any()).Return("hash123", nil).Times(1)
			mockStore.EXPECT().Get("build").Return(nil, nil).Times(1)
			mockExecutor.EXPECT().Execute(
				gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
			).Return(nil).Times(1)
			mockStore.EXPECT().Put(gomock.Any()).Return(nil).Times(1)

			cli.SetArgs(tt.args)

			err := cli.Execute(context.Background())
			if err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}
