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

type testCLI struct {
	CLI          *commands.CLI
	Ctrl         *gomock.Controller
	MockLoader   *mocks.MockConfigLoader
	MockExecutor *mocks.MockExecutor
	MockStore    *mocks.MockBuildInfoStore
	MockHasher   *mocks.MockHasher
	MockResolver *mocks.MockInputResolver
	MockLogger   *mocks.MockLogger
}

func setupTestCLI(t *testing.T) *testCLI {
	t.Helper()

	ctrl := gomock.NewController(t)
	mockLoader := mocks.NewMockConfigLoader(ctrl)
	mockExecutor := mocks.NewMockExecutor(ctrl)
	mockStore := mocks.NewMockBuildInfoStore(ctrl)
	mockHasher := mocks.NewMockHasher(ctrl)
	mockResolver := mocks.NewMockInputResolver(ctrl)
	mockEnvFactory := mocks.NewMockEnvironmentFactory(ctrl)
	mockLogger := mocks.NewMockLogger(ctrl)

	a := app.New(mockLoader, mockExecutor, mockLogger, mockStore, mockHasher, mockResolver, mockEnvFactory).
		WithTeaOptions(tea.WithInput(nil), tea.WithOutput(io.Discard))

	return &testCLI{
		CLI:          commands.New(a),
		Ctrl:         ctrl,
		MockLoader:   mockLoader,
		MockExecutor: mockExecutor,
		MockStore:    mockStore,
		MockHasher:   mockHasher,
		MockResolver: mockResolver,
		MockLogger:   mockLogger,
	}
}

func setupSimpleTestCLI(t *testing.T) (*commands.CLI, *gomock.Controller, *mocks.MockLogger) {
	t.Helper()

	ctrl := gomock.NewController(t)
	mockLoader := mocks.NewMockConfigLoader(ctrl)
	mockExecutor := mocks.NewMockExecutor(ctrl)
	mockStore := mocks.NewMockBuildInfoStore(ctrl)
	mockHasher := mocks.NewMockHasher(ctrl)
	mockResolver := mocks.NewMockInputResolver(ctrl)
	mockEnvFactory := mocks.NewMockEnvironmentFactory(ctrl)
	mockLogger := mocks.NewMockLogger(ctrl)

	a := app.New(mockLoader, mockExecutor, mockLogger, mockStore, mockHasher, mockResolver, mockEnvFactory).
		WithTeaOptions(tea.WithInput(nil), tea.WithOutput(io.Discard))

	return commands.New(a), ctrl, mockLogger
}

func TestRun_Success(t *testing.T) {
	tc := setupTestCLI(t)

	g := domain.NewGraph()
	buildTask := &domain.Task{Name: domain.NewInternedString("build"), WorkingDir: domain.NewInternedString("Root")}
	_ = g.AddTask(buildTask)

	tc.MockLogger.EXPECT().SetJSON(false).Times(1)
	tc.MockLoader.EXPECT().Load(".").Return(g, nil).Times(1)
	tc.MockResolver.EXPECT().ResolveInputs(gomock.Any(), gomock.Any()).Return([]string{}, nil).Times(1)
	tc.MockHasher.EXPECT().ComputeInputHash(gomock.Any(), gomock.Any(), gomock.Any()).Return("hash123", nil).Times(1)
	tc.MockStore.EXPECT().Get("build").Return(nil, nil).Times(1)
	tc.MockExecutor.EXPECT().Execute(
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
	).Return(nil).Times(1)
	tc.MockStore.EXPECT().Put(gomock.Any()).Return(nil).Times(1)

	tc.CLI.SetArgs([]string{"run", "build"})

	err := tc.CLI.Execute(context.Background())
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestRun_NoTargets(t *testing.T) {
	cli, _, mockLogger := setupSimpleTestCLI(t)

	mockLogger.EXPECT().SetJSON(false).Times(1)

	cli.SetArgs([]string{"run"})

	err := cli.Execute(context.Background())
	if err != nil {
		t.Errorf("Expected no error for no targets, got: %v", err)
	}
}

func TestRoot_Help(t *testing.T) {
	cli, _, _ := setupSimpleTestCLI(t)

	cli.SetArgs([]string{"--help"})

	err := cli.Execute(context.Background())
	if err != nil {
		t.Errorf("Expected no error for help, got: %v", err)
	}
}

func TestRoot_Version(t *testing.T) {
	cli, _, _ := setupSimpleTestCLI(t)

	cli.SetArgs([]string{"--version"})

	err := cli.Execute(context.Background())
	if err != nil {
		t.Errorf("Expected no error for version, got: %v", err)
	}
}

func TestVersionCmd(t *testing.T) {
	cli, _, mockLogger := setupSimpleTestCLI(t)

	mockLogger.EXPECT().SetJSON(false).Times(1)

	cli.SetArgs([]string{"version"})

	err := cli.Execute(context.Background())
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
	mockLogger.EXPECT().SetJSON(false).Times(1)
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
	mockLogger.EXPECT().SetJSON(false).Times(1)
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
	mockLogger.EXPECT().SetJSON(false).Times(1)
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
			tc := setupTestCLI(t)

			g := domain.NewGraph()
			g.SetRoot(".")
			buildTask := &domain.Task{Name: domain.NewInternedString("build"), WorkingDir: domain.NewInternedString("Root")}
			_ = g.AddTask(buildTask)

			tc.MockLogger.EXPECT().SetJSON(false).Times(1)
			tc.MockLoader.EXPECT().Load(".").Return(g, nil).Times(1)
			tc.MockResolver.EXPECT().ResolveInputs(gomock.Any(), ".").Return([]string{}, nil).Times(1)
			tc.MockHasher.EXPECT().ComputeInputHash(gomock.Any(), gomock.Any(), gomock.Any()).Return("hash123", nil).Times(1)
			tc.MockStore.EXPECT().Get("build").Return(nil, nil).Times(1)
			tc.MockExecutor.EXPECT().Execute(
				gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
			).Return(nil).Times(1)
			tc.MockStore.EXPECT().Put(gomock.Any()).Return(nil).Times(1)

			tc.CLI.SetArgs(tt.args)

			err := tc.CLI.Execute(context.Background())
			if err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}

func TestRun_JSONFlag(t *testing.T) {
	tc := setupTestCLI(t)

	g := domain.NewGraph()
	buildTask := &domain.Task{Name: domain.NewInternedString("build"), WorkingDir: domain.NewInternedString("Root")}
	_ = g.AddTask(buildTask)

	tc.MockLogger.EXPECT().SetJSON(true).Times(1)
	tc.MockLoader.EXPECT().Load(".").Return(g, nil).Times(1)
	tc.MockResolver.EXPECT().ResolveInputs(gomock.Any(), gomock.Any()).Return([]string{}, nil).Times(1)
	tc.MockHasher.EXPECT().ComputeInputHash(gomock.Any(), gomock.Any(), gomock.Any()).Return("hash123", nil).Times(1)
	tc.MockStore.EXPECT().Get("build").Return(nil, nil).Times(1)
	tc.MockExecutor.EXPECT().Execute(
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
	).Return(nil).Times(1)
	tc.MockStore.EXPECT().Put(gomock.Any()).Return(nil).Times(1)

	tc.CLI.SetArgs([]string{"--json", "run", "build"})

	err := tc.CLI.Execute(context.Background())
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}
