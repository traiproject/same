package app_test

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/synctest"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
	"go.trai.ch/same/internal/app"
	"go.trai.ch/same/internal/core/domain"
	"go.trai.ch/same/internal/core/ports/mocks"
	"go.uber.org/mock/gomock"
)

func TestApp_Build(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Use a temporary directory for the test
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
		defer ctrl.Finish()

		mockLoader := mocks.NewMockConfigLoader(ctrl)
		mockExecutor := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockEnvFactory := mocks.NewMockEnvironmentFactory(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)

		// Setup Graph
		g := domain.NewGraph()
		g.SetRoot(".")
		task := &domain.Task{Name: domain.NewInternedString("task1"), WorkingDir: domain.NewInternedString("Root")}
		_ = g.AddTask(task)

		// Setup App
		a := app.New(mockLoader, mockExecutor, mockLogger, mockStore, mockHasher, mockResolver, mockEnvFactory).
			WithTeaOptions(
				tea.WithInput(strings.NewReader("")),
				tea.WithOutput(io.Discard),
				tea.WithoutSignalHandler(),
				tea.WithoutRenderer(),
			).
			WithDisableTick()

		mockResolver.EXPECT().ResolveInputs(gomock.Any(), ".").Return([]string{}, nil).AnyTimes()
		// Expectations
		mockLoader.EXPECT().Load(".").Return(g, nil)
		mockHasher.EXPECT().ComputeInputHash(task, nil, []string{}).Return("hash", nil)
		mockStore.EXPECT().Get("task1").Return(nil, nil)
		mockExecutor.EXPECT().Execute(gomock.Any(), task, gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		mockStore.EXPECT().Put(gomock.Any()).Return(nil)

		// Run
		err = a.Run(context.Background(), []string{"task1"}, app.RunOptions{NoCache: false})
		// Assert
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})
}

func TestApp_Run_NoTargets(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Use a temporary directory for the test
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
		defer ctrl.Finish()

		mockLoader := mocks.NewMockConfigLoader(ctrl)
		mockExecutor := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockEnvFactory := mocks.NewMockEnvironmentFactory(ctrl)

		// Setup App
		mockLogger := mocks.NewMockLogger(ctrl)
		a := app.New(mockLoader, mockExecutor, mockLogger, mockStore, mockHasher, mockResolver, mockEnvFactory).
			WithTeaOptions(
				tea.WithInput(strings.NewReader("")),
				tea.WithOutput(io.Discard),
				tea.WithoutSignalHandler(),
			).
			WithDisableTick()

		// Expectations
		mockLoader.EXPECT().Load(".").Return(domain.NewGraph(), nil)

		// Execute
		err = a.Run(context.Background(), nil, app.RunOptions{NoCache: false})
		if err == nil {
			t.Error("Expected error, got nil")
		}
		if err.Error() != "no targets specified" {
			t.Errorf("Expected 'no targets specified', got '%v'", err)
		}
	})
}

func TestApp_Run_ConfigLoaderError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Use a temporary directory for the test
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
		defer ctrl.Finish()

		mockLoader := mocks.NewMockConfigLoader(ctrl)
		mockExecutor := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockEnvFactory := mocks.NewMockEnvironmentFactory(ctrl)

		// Setup App
		mockLogger := mocks.NewMockLogger(ctrl)
		a := app.New(mockLoader, mockExecutor, mockLogger, mockStore, mockHasher, mockResolver, mockEnvFactory).
			WithTeaOptions(
				tea.WithInput(strings.NewReader("")),
				tea.WithOutput(io.Discard),
				tea.WithoutSignalHandler(),
			).
			WithDisableTick()

		// Expectations - loader fails
		mockLoader.EXPECT().Load(".").Return(nil, errors.New("config load error"))

		// Execute
		err = a.Run(context.Background(), []string{"task1"}, app.RunOptions{NoCache: false})
		if err == nil {
			t.Error("Expected error, got nil")
		}
		if !errors.Is(err, errors.New("config load error")) {
			// Check that error contains our message
			if err.Error() == "" || !strings.Contains(err.Error(), "failed to load configuration") {
				t.Errorf("Expected error to contain 'failed to load configuration', got '%v'", err)
			}
		}
	})
}

func TestApp_Run_BuildExecutionFailed(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Use a temporary directory for the test
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
		defer ctrl.Finish()

		mockLoader := mocks.NewMockConfigLoader(ctrl)
		mockExecutor := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockEnvFactory := mocks.NewMockEnvironmentFactory(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)

		// Setup Graph
		g := domain.NewGraph()
		g.SetRoot(".")
		task := &domain.Task{Name: domain.NewInternedString("task1"), WorkingDir: domain.NewInternedString("Root")}
		_ = g.AddTask(task)

		// Setup App
		a := app.New(mockLoader, mockExecutor, mockLogger, mockStore, mockHasher, mockResolver, mockEnvFactory).
			WithTeaOptions(
				tea.WithInput(strings.NewReader("")),
				tea.WithOutput(io.Discard),
				tea.WithoutSignalHandler(),
				tea.WithoutRenderer(),
			).
			WithDisableTick()

		mockResolver.EXPECT().ResolveInputs(gomock.Any(), ".").Return([]string{}, nil).AnyTimes()
		// Expectations
		mockLoader.EXPECT().Load(".").Return(g, nil)
		mockHasher.EXPECT().ComputeInputHash(task, nil, []string{}).Return("hash", nil)
		mockStore.EXPECT().Get("task1").Return(nil, nil)
		// Mock Executor failure
		mockExecutor.EXPECT().Execute(gomock.Any(), task, gomock.Any(), gomock.Any(), gomock.Any()).
			Return(errors.New("command failed"))

		// Run
		err = a.Run(context.Background(), []string{"task1"}, app.RunOptions{NoCache: false})
		// Assert
		if err == nil {
			t.Error("Expected error, got nil")
		}
		if !errors.Is(err, domain.ErrBuildExecutionFailed) {
			t.Errorf("Expected error to wrap ErrBuildExecutionFailed, got: %v", err)
		}
	})
}

//nolint:cyclop // Table-driven test
func TestApp_Clean(t *testing.T) {
	tests := []struct {
		name    string
		options app.CleanOptions
		setup   func(createDir func(string))
		check   func(t *testing.T, exists func(string) bool)
	}{
		{
			name:    "Clean Build Only",
			options: app.CleanOptions{Build: true, Tools: false},
			setup: func(createDir func(string)) {
				createDir(domain.DefaultStorePath())
				createDir(domain.DefaultNixHubCachePath())
			},
			check: func(t *testing.T, exists func(string) bool) {
				t.Helper()
				if exists(domain.DefaultStorePath()) {
					t.Error("Store should be removed")
				}
				if !exists(domain.DefaultNixHubCachePath()) {
					t.Error("Nix cache should remain")
				}
			},
		},
		{
			name:    "Clean Tools Only",
			options: app.CleanOptions{Build: false, Tools: true},
			setup: func(createDir func(string)) {
				createDir(domain.DefaultStorePath())
				createDir(domain.DefaultNixHubCachePath())
				createDir(domain.DefaultEnvCachePath())
			},
			check: func(t *testing.T, exists func(string) bool) {
				t.Helper()
				if !exists(domain.DefaultStorePath()) {
					t.Error("Store should remain")
				}
				if exists(domain.DefaultNixHubCachePath()) {
					t.Error("Nix cache should be removed")
				}
				if exists(domain.DefaultEnvCachePath()) {
					t.Error("Env cache should be removed")
				}
			},
		},
		{
			name:    "Clean All",
			options: app.CleanOptions{Build: true, Tools: true},
			setup: func(createDir func(string)) {
				createDir(domain.DefaultStorePath())
				createDir(domain.DefaultNixHubCachePath())
			},
			check: func(t *testing.T, exists func(string) bool) {
				t.Helper()
				if exists(domain.DefaultStorePath()) {
					t.Error("Store should be removed")
				}
				if exists(domain.DefaultNixHubCachePath()) {
					t.Error("Nix cache should be removed")
				}
			},
		},
		{
			name:    "Idempotent",
			options: app.CleanOptions{Build: true, Tools: true},
			setup:   func(_ func(string)) {}, // Nothing created
			check: func(_ *testing.T, _ func(string) bool) {
				// Should not error
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				// Use a temporary directory for the test
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

				// Helper to create directories
				createDir := func(path string) {
					if mkdirErr := os.MkdirAll(path, domain.DirPerm); mkdirErr != nil {
						t.Fatalf("Failed to create directory %s: %v", path, mkdirErr)
					}
				}

				// Helper to check if directory exists
				exists := func(path string) bool {
					_, statErr := os.Stat(path)
					return statErr == nil
				}

				// Clean everything before setup (technically fresh tmpDir but good practice)
				_ = os.RemoveAll(domain.DefaultSamePath())

				tt.setup(createDir)

				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				mockLogger := mocks.NewMockLogger(ctrl)
				// We expect some logs, but we can be loose or strict.
				// Let's just allow any Info calls.
				mockLogger.EXPECT().Info(gomock.Any()).AnyTimes()

				// Null dependencies for others
				a := app.New(nil, nil, mockLogger, nil, nil, nil, nil)

				err = a.Clean(context.Background(), tt.options)
				if err != nil {
					t.Errorf("Clean() error = %v", err)
				}
				tt.check(t, exists)
			})
		})
	}
}

func TestApp_Run_LinearMode(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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
		defer ctrl.Finish()

		mockLoader := mocks.NewMockConfigLoader(ctrl)
		mockExecutor := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockEnvFactory := mocks.NewMockEnvironmentFactory(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)

		g := domain.NewGraph()
		g.SetRoot(".")
		task := &domain.Task{Name: domain.NewInternedString("task1"), WorkingDir: domain.NewInternedString("Root")}
		_ = g.AddTask(task)

		a := app.New(mockLoader, mockExecutor, mockLogger, mockStore, mockHasher, mockResolver, mockEnvFactory).
			WithTeaOptions(
				tea.WithInput(strings.NewReader("")),
				tea.WithOutput(io.Discard),
				tea.WithoutSignalHandler(),
				tea.WithoutRenderer(),
			).
			WithDisableTick()

		mockResolver.EXPECT().ResolveInputs(gomock.Any(), ".").Return([]string{}, nil).AnyTimes()
		mockLoader.EXPECT().Load(".").Return(g, nil)
		mockHasher.EXPECT().ComputeInputHash(task, nil, []string{}).Return("hash", nil)
		mockStore.EXPECT().Get("task1").Return(nil, nil)
		mockExecutor.EXPECT().Execute(gomock.Any(), task, gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		mockStore.EXPECT().Put(gomock.Any()).Return(nil)

		err = a.Run(context.Background(), []string{"task1"}, app.RunOptions{
			NoCache:    false,
			OutputMode: "linear",
		})
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})
}

func TestApp_Run_InspectMode(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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
		defer ctrl.Finish()

		mockLoader := mocks.NewMockConfigLoader(ctrl)
		mockExecutor := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockEnvFactory := mocks.NewMockEnvironmentFactory(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)

		g := domain.NewGraph()
		g.SetRoot(".")
		task := &domain.Task{Name: domain.NewInternedString("task1"), WorkingDir: domain.NewInternedString("Root")}
		_ = g.AddTask(task)

		a := app.New(mockLoader, mockExecutor, mockLogger, mockStore, mockHasher, mockResolver, mockEnvFactory).
			WithTeaOptions(
				tea.WithInput(strings.NewReader("q")),
				tea.WithOutput(io.Discard),
				tea.WithoutSignalHandler(),
			).
			WithDisableTick()

		mockResolver.EXPECT().ResolveInputs(gomock.Any(), ".").Return([]string{}, nil).AnyTimes()
		mockLoader.EXPECT().Load(".").Return(g, nil)
		mockHasher.EXPECT().ComputeInputHash(task, nil, []string{}).Return("hash", nil)
		mockStore.EXPECT().Get("task1").Return(nil, nil)
		mockExecutor.EXPECT().Execute(gomock.Any(), task, gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		mockStore.EXPECT().Put(gomock.Any()).Return(nil)

		err = a.Run(context.Background(), []string{"task1"}, app.RunOptions{
			NoCache: false,
			Inspect: true,
		})
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})
}

func TestApp_Run_TaskNotFound(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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
		defer ctrl.Finish()

		mockLoader := mocks.NewMockConfigLoader(ctrl)
		mockExecutor := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockEnvFactory := mocks.NewMockEnvironmentFactory(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)

		g := domain.NewGraph()
		g.SetRoot(".")

		a := app.New(mockLoader, mockExecutor, mockLogger, mockStore, mockHasher, mockResolver, mockEnvFactory).
			WithTeaOptions(
				tea.WithInput(strings.NewReader("")),
				tea.WithOutput(io.Discard),
				tea.WithoutSignalHandler(),
			).
			WithDisableTick()

		mockResolver.EXPECT().ResolveInputs(gomock.Any(), ".").Return([]string{}, nil).AnyTimes()
		mockLoader.EXPECT().Load(".").Return(g, nil)

		err = a.Run(context.Background(), []string{"nonexistent"}, app.RunOptions{NoCache: false})
		if err == nil {
			t.Error("Expected error, got nil")
		}
		// TaskNotFound is wrapped in ErrBuildExecutionFailed
		if !errors.Is(err, domain.ErrBuildExecutionFailed) {
			t.Errorf("Expected error to wrap ErrBuildExecutionFailed, got: %v", err)
		}
	})
}

func TestApp_Run_HasherError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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
		defer ctrl.Finish()

		mockLoader := mocks.NewMockConfigLoader(ctrl)
		mockExecutor := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockEnvFactory := mocks.NewMockEnvironmentFactory(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)

		g := domain.NewGraph()
		g.SetRoot(".")
		task := &domain.Task{Name: domain.NewInternedString("task1"), WorkingDir: domain.NewInternedString("Root")}
		_ = g.AddTask(task)

		a := app.New(mockLoader, mockExecutor, mockLogger, mockStore, mockHasher, mockResolver, mockEnvFactory).
			WithTeaOptions(
				tea.WithInput(strings.NewReader("")),
				tea.WithOutput(io.Discard),
				tea.WithoutSignalHandler(),
				tea.WithoutRenderer(),
			).
			WithDisableTick()

		mockResolver.EXPECT().ResolveInputs(gomock.Any(), ".").Return([]string{}, nil).AnyTimes()
		mockLoader.EXPECT().Load(".").Return(g, nil)
		mockHasher.EXPECT().ComputeInputHash(task, nil, []string{}).Return("", errors.New("hash computation failed"))

		err = a.Run(context.Background(), []string{"task1"}, app.RunOptions{NoCache: false})
		if err == nil {
			t.Error("Expected error, got nil")
		}
		if !errors.Is(err, domain.ErrBuildExecutionFailed) {
			t.Errorf("Expected error to wrap ErrBuildExecutionFailed, got: %v", err)
		}
	})
}

func TestApp_Run_StoreGetError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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
		defer ctrl.Finish()

		mockLoader := mocks.NewMockConfigLoader(ctrl)
		mockExecutor := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockEnvFactory := mocks.NewMockEnvironmentFactory(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)

		g := domain.NewGraph()
		g.SetRoot(".")
		task := &domain.Task{Name: domain.NewInternedString("task1"), WorkingDir: domain.NewInternedString("Root")}
		_ = g.AddTask(task)

		a := app.New(mockLoader, mockExecutor, mockLogger, mockStore, mockHasher, mockResolver, mockEnvFactory).
			WithTeaOptions(
				tea.WithInput(strings.NewReader("")),
				tea.WithOutput(io.Discard),
				tea.WithoutSignalHandler(),
				tea.WithoutRenderer(),
			).
			WithDisableTick()

		mockResolver.EXPECT().ResolveInputs(gomock.Any(), ".").Return([]string{}, nil).AnyTimes()
		mockLoader.EXPECT().Load(".").Return(g, nil)
		mockHasher.EXPECT().ComputeInputHash(task, nil, []string{}).Return("hash", nil)
		mockStore.EXPECT().Get("task1").Return(nil, errors.New("store read error"))

		err = a.Run(context.Background(), []string{"task1"}, app.RunOptions{NoCache: false})
		if err == nil {
			t.Error("Expected error, got nil")
		}
		if !errors.Is(err, domain.ErrBuildExecutionFailed) {
			t.Errorf("Expected error to wrap ErrBuildExecutionFailed, got: %v", err)
		}
	})
}

func TestApp_Run_InputResolverError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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
		defer ctrl.Finish()

		mockLoader := mocks.NewMockConfigLoader(ctrl)
		mockExecutor := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockEnvFactory := mocks.NewMockEnvironmentFactory(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)

		g := domain.NewGraph()
		g.SetRoot(".")
		task := &domain.Task{Name: domain.NewInternedString("task1"), WorkingDir: domain.NewInternedString("Root")}
		_ = g.AddTask(task)

		a := app.New(mockLoader, mockExecutor, mockLogger, mockStore, mockHasher, mockResolver, mockEnvFactory).
			WithTeaOptions(
				tea.WithInput(strings.NewReader("")),
				tea.WithOutput(io.Discard),
				tea.WithoutSignalHandler(),
				tea.WithoutRenderer(),
			).
			WithDisableTick()

		mockResolver.EXPECT().ResolveInputs(gomock.Any(), ".").Return(nil, errors.New("input resolution failed"))
		mockLoader.EXPECT().Load(".").Return(g, nil)

		err = a.Run(context.Background(), []string{"task1"}, app.RunOptions{NoCache: false})
		if err == nil {
			t.Error("Expected error, got nil")
		}
		if !errors.Is(err, domain.ErrBuildExecutionFailed) {
			t.Errorf("Expected error to wrap ErrBuildExecutionFailed, got: %v", err)
		}
	})
}

func TestApp_Run_EnvironmentFactoryError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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
		defer ctrl.Finish()

		mockLoader := mocks.NewMockConfigLoader(ctrl)
		mockExecutor := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockEnvFactory := mocks.NewMockEnvironmentFactory(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)

		g := domain.NewGraph()
		g.SetRoot(".")
		tools := map[string]string{"go": "go@1.23"}
		task := &domain.Task{
			Name:       domain.NewInternedString("task1"),
			WorkingDir: domain.NewInternedString("Root"),
			Tools:      tools,
		}
		_ = g.AddTask(task)

		a := app.New(mockLoader, mockExecutor, mockLogger, mockStore, mockHasher, mockResolver, mockEnvFactory).
			WithTeaOptions(
				tea.WithInput(strings.NewReader("")),
				tea.WithOutput(io.Discard),
				tea.WithoutSignalHandler(),
				tea.WithoutRenderer(),
			).
			WithDisableTick()

		// Environment factory is called BEFORE hasher and store in Phase 1 (prepareEnvironments)
		mockResolver.EXPECT().ResolveInputs(gomock.Any(), ".").Return([]string{}, nil).AnyTimes()
		mockLoader.EXPECT().Load(".").Return(g, nil)
		mockEnvFactory.EXPECT().GetEnvironment(gomock.Any(), tools).Return(nil, errors.New("environment resolution failed"))

		err = a.Run(context.Background(), []string{"task1"}, app.RunOptions{NoCache: false})
		if err == nil {
			t.Error("Expected error, got nil")
		}
		if !errors.Is(err, domain.ErrBuildExecutionFailed) {
			t.Errorf("Expected error to wrap ErrBuildExecutionFailed, got: %v", err)
		}
	})
}

func TestApp_Run_CacheHit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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
		defer ctrl.Finish()

		mockLoader := mocks.NewMockConfigLoader(ctrl)
		mockExecutor := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockEnvFactory := mocks.NewMockEnvironmentFactory(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)

		g := domain.NewGraph()
		g.SetRoot(".")
		task := &domain.Task{Name: domain.NewInternedString("task1"), WorkingDir: domain.NewInternedString("Root")}
		_ = g.AddTask(task)

		a := app.New(mockLoader, mockExecutor, mockLogger, mockStore, mockHasher, mockResolver, mockEnvFactory).
			WithTeaOptions(
				tea.WithInput(strings.NewReader("")),
				tea.WithOutput(io.Discard),
				tea.WithoutSignalHandler(),
				tea.WithoutRenderer(),
			).
			WithDisableTick()

		mockResolver.EXPECT().ResolveInputs(gomock.Any(), ".").Return([]string{}, nil).AnyTimes()
		mockLoader.EXPECT().Load(".").Return(g, nil)
		mockHasher.EXPECT().ComputeInputHash(task, nil, []string{}).Return("hash123", nil)
		mockStore.EXPECT().Get("task1").Return(&domain.BuildInfo{
			TaskName:   "task1",
			InputHash:  "hash123",
			OutputHash: "",
		}, nil)
		// Executor should NOT be called for cache hit

		err = a.Run(context.Background(), []string{"task1"}, app.RunOptions{NoCache: false})
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})
}

func TestApp_Run_NoCacheMode(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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
		defer ctrl.Finish()

		mockLoader := mocks.NewMockConfigLoader(ctrl)
		mockExecutor := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockEnvFactory := mocks.NewMockEnvironmentFactory(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)

		g := domain.NewGraph()
		g.SetRoot(".")
		task := &domain.Task{Name: domain.NewInternedString("task1"), WorkingDir: domain.NewInternedString("Root")}
		_ = g.AddTask(task)

		a := app.New(mockLoader, mockExecutor, mockLogger, mockStore, mockHasher, mockResolver, mockEnvFactory).
			WithTeaOptions(
				tea.WithInput(strings.NewReader("")),
				tea.WithOutput(io.Discard),
				tea.WithoutSignalHandler(),
				tea.WithoutRenderer(),
			).
			WithDisableTick()

		mockResolver.EXPECT().ResolveInputs(gomock.Any(), ".").Return([]string{}, nil).AnyTimes()
		mockLoader.EXPECT().Load(".").Return(g, nil)
		// With NoCache=true, ComputeInputHash is called but store.Get is skipped
		mockHasher.EXPECT().ComputeInputHash(task, nil, []string{}).Return("hash", nil)
		mockExecutor.EXPECT().Execute(gomock.Any(), task, gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		mockStore.EXPECT().Put(gomock.Any()).Return(nil)

		err = a.Run(context.Background(), []string{"task1"}, app.RunOptions{NoCache: true})
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})
}

func TestApp_Run_RebuildAlwaysStrategy(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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
		defer ctrl.Finish()

		mockLoader := mocks.NewMockConfigLoader(ctrl)
		mockExecutor := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockEnvFactory := mocks.NewMockEnvironmentFactory(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)

		g := domain.NewGraph()
		g.SetRoot(".")
		task := &domain.Task{
			Name:            domain.NewInternedString("task1"),
			WorkingDir:      domain.NewInternedString("Root"),
			RebuildStrategy: domain.RebuildAlways,
		}
		_ = g.AddTask(task)

		a := app.New(mockLoader, mockExecutor, mockLogger, mockStore, mockHasher, mockResolver, mockEnvFactory).
			WithTeaOptions(
				tea.WithInput(strings.NewReader("")),
				tea.WithOutput(io.Discard),
				tea.WithoutSignalHandler(),
				tea.WithoutRenderer(),
			).
			WithDisableTick()

		mockResolver.EXPECT().ResolveInputs(gomock.Any(), ".").Return([]string{}, nil).AnyTimes()
		mockLoader.EXPECT().Load(".").Return(g, nil)
		// With RebuildAlways, store.Get is skipped
		mockHasher.EXPECT().ComputeInputHash(task, nil, []string{}).Return("hash", nil)
		mockExecutor.EXPECT().Execute(gomock.Any(), task, gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		mockStore.EXPECT().Put(gomock.Any()).Return(nil)

		err = a.Run(context.Background(), []string{"task1"}, app.RunOptions{NoCache: false})
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})
}

func TestApp_Run_MultipleTasks(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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
		defer ctrl.Finish()

		mockLoader := mocks.NewMockConfigLoader(ctrl)
		mockExecutor := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockEnvFactory := mocks.NewMockEnvironmentFactory(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)

		g := domain.NewGraph()
		g.SetRoot(".")
		task1 := &domain.Task{Name: domain.NewInternedString("task1"), WorkingDir: domain.NewInternedString("Root")}
		task2 := &domain.Task{Name: domain.NewInternedString("task2"), WorkingDir: domain.NewInternedString("Root")}
		_ = g.AddTask(task1)
		_ = g.AddTask(task2)

		a := app.New(mockLoader, mockExecutor, mockLogger, mockStore, mockHasher, mockResolver, mockEnvFactory).
			WithTeaOptions(
				tea.WithInput(strings.NewReader("")),
				tea.WithOutput(io.Discard),
				tea.WithoutSignalHandler(),
				tea.WithoutRenderer(),
			).
			WithDisableTick()

		mockResolver.EXPECT().ResolveInputs(gomock.Any(), ".").Return([]string{}, nil).AnyTimes()
		mockLoader.EXPECT().Load(".").Return(g, nil)
		mockHasher.EXPECT().ComputeInputHash(task1, nil, []string{}).Return("hash1", nil)
		mockHasher.EXPECT().ComputeInputHash(task2, nil, []string{}).Return("hash2", nil)
		mockStore.EXPECT().Get("task1").Return(nil, nil)
		mockStore.EXPECT().Get("task2").Return(nil, nil)
		mockExecutor.EXPECT().Execute(gomock.Any(), task1, gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		mockExecutor.EXPECT().Execute(gomock.Any(), task2, gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		mockStore.EXPECT().Put(gomock.Any()).Return(nil).Times(2)

		err = a.Run(context.Background(), []string{"task1", "task2"}, app.RunOptions{NoCache: false})
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})
}

func TestApp_Run_TaskWithDependencies(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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
		defer ctrl.Finish()

		mockLoader := mocks.NewMockConfigLoader(ctrl)
		mockExecutor := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockEnvFactory := mocks.NewMockEnvironmentFactory(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)

		g := domain.NewGraph()
		g.SetRoot(".")
		depTask := &domain.Task{Name: domain.NewInternedString("dep"), WorkingDir: domain.NewInternedString("Root")}
		mainTask := &domain.Task{
			Name:         domain.NewInternedString("main"),
			WorkingDir:   domain.NewInternedString("Root"),
			Dependencies: []domain.InternedString{domain.NewInternedString("dep")},
		}
		_ = g.AddTask(depTask)
		_ = g.AddTask(mainTask)

		a := app.New(mockLoader, mockExecutor, mockLogger, mockStore, mockHasher, mockResolver, mockEnvFactory).
			WithTeaOptions(
				tea.WithInput(strings.NewReader("")),
				tea.WithOutput(io.Discard),
				tea.WithoutSignalHandler(),
				tea.WithoutRenderer(),
			).
			WithDisableTick()

		mockResolver.EXPECT().ResolveInputs(gomock.Any(), ".").Return([]string{}, nil).AnyTimes()
		mockLoader.EXPECT().Load(".").Return(g, nil)
		mockHasher.EXPECT().ComputeInputHash(depTask, nil, []string{}).Return("hash1", nil)
		mockHasher.EXPECT().ComputeInputHash(mainTask, nil, []string{}).Return("hash2", nil)
		mockStore.EXPECT().Get("dep").Return(nil, nil)
		mockStore.EXPECT().Get("main").Return(nil, nil)
		mockExecutor.EXPECT().Execute(gomock.Any(), depTask, gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		mockExecutor.EXPECT().Execute(gomock.Any(), mainTask, gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		mockStore.EXPECT().Put(gomock.Any()).Return(nil).Times(2)

		err = a.Run(context.Background(), []string{"main"}, app.RunOptions{NoCache: false})
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})
}

func TestApp_Run_AllTarget(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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
		defer ctrl.Finish()

		mockLoader := mocks.NewMockConfigLoader(ctrl)
		mockExecutor := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockEnvFactory := mocks.NewMockEnvironmentFactory(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)

		g := domain.NewGraph()
		g.SetRoot(".")
		task1 := &domain.Task{Name: domain.NewInternedString("task1"), WorkingDir: domain.NewInternedString("Root")}
		task2 := &domain.Task{Name: domain.NewInternedString("task2"), WorkingDir: domain.NewInternedString("Root")}
		_ = g.AddTask(task1)
		_ = g.AddTask(task2)

		a := app.New(mockLoader, mockExecutor, mockLogger, mockStore, mockHasher, mockResolver, mockEnvFactory).
			WithTeaOptions(
				tea.WithInput(strings.NewReader("")),
				tea.WithOutput(io.Discard),
				tea.WithoutSignalHandler(),
				tea.WithoutRenderer(),
			).
			WithDisableTick()

		mockResolver.EXPECT().ResolveInputs(gomock.Any(), ".").Return([]string{}, nil).AnyTimes()
		mockLoader.EXPECT().Load(".").Return(g, nil)
		mockHasher.EXPECT().ComputeInputHash(task1, nil, []string{}).Return("hash1", nil)
		mockHasher.EXPECT().ComputeInputHash(task2, nil, []string{}).Return("hash2", nil)
		mockStore.EXPECT().Get("task1").Return(nil, nil)
		mockStore.EXPECT().Get("task2").Return(nil, nil)
		mockExecutor.EXPECT().Execute(gomock.Any(), task1, gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		mockExecutor.EXPECT().Execute(gomock.Any(), task2, gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		mockStore.EXPECT().Put(gomock.Any()).Return(nil).Times(2)

		err = a.Run(context.Background(), []string{"all"}, app.RunOptions{NoCache: false})
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})
}

func TestApp_Run_ContextCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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
		defer ctrl.Finish()

		mockLoader := mocks.NewMockConfigLoader(ctrl)
		mockExecutor := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockEnvFactory := mocks.NewMockEnvironmentFactory(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)

		g := domain.NewGraph()
		g.SetRoot(".")
		task := &domain.Task{Name: domain.NewInternedString("task1"), WorkingDir: domain.NewInternedString("Root")}
		_ = g.AddTask(task)

		a := app.New(mockLoader, mockExecutor, mockLogger, mockStore, mockHasher, mockResolver, mockEnvFactory).
			WithTeaOptions(
				tea.WithInput(strings.NewReader("")),
				tea.WithOutput(io.Discard),
				tea.WithoutSignalHandler(),
				tea.WithoutRenderer(),
			).
			WithDisableTick()

		ctx, cancel := context.WithCancel(context.Background())

		mockResolver.EXPECT().ResolveInputs(gomock.Any(), ".").Return([]string{}, nil).AnyTimes()
		mockLoader.EXPECT().Load(".").Return(g, nil)
		mockHasher.EXPECT().ComputeInputHash(task, nil, []string{}).Return("hash", nil)
		mockStore.EXPECT().Get("task1").Return(nil, nil)
		// Cancel context before execution completes
		mockExecutor.EXPECT().Execute(gomock.Any(), task, gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, _ *domain.Task, _ []string, _, _ interface{}) error {
				cancel()
				return ctx.Err()
			})

		err = a.Run(ctx, []string{"task1"}, app.RunOptions{NoCache: false})
		if err == nil {
			t.Fatal("Expected error due to context cancellation, got nil")
		}
		if !errors.Is(err, context.Canceled) && !errors.Is(err, domain.ErrBuildExecutionFailed) {
			t.Errorf("Expected context cancellation or build execution error, got: %v", err)
		}
	})
}

func TestApp_Clean_NoOptions(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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

		// Create directories that should remain
		require.NoError(t, os.MkdirAll(domain.DefaultStorePath(), domain.DirPerm))
		require.NoError(t, os.MkdirAll(domain.DefaultNixHubCachePath(), domain.DirPerm))

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockLogger := mocks.NewMockLogger(ctrl)

		a := app.New(nil, nil, mockLogger, nil, nil, nil, nil)

		// Clean with no options - should not remove anything
		err = a.Clean(context.Background(), app.CleanOptions{Build: false, Tools: false})
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		// Verify directories still exist
		if _, statErr := os.Stat(domain.DefaultStorePath()); statErr != nil {
			t.Error("Store should still exist when CleanOptions is empty")
		}
		if _, statErr := os.Stat(domain.DefaultNixHubCachePath()); statErr != nil {
			t.Error("Nix cache should still exist when CleanOptions is empty")
		}
	})
}

func TestApp_Run_TaskWithTools(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
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
		defer ctrl.Finish()

		mockLoader := mocks.NewMockConfigLoader(ctrl)
		mockExecutor := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockEnvFactory := mocks.NewMockEnvironmentFactory(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)

		g := domain.NewGraph()
		g.SetRoot(".")
		tools := map[string]string{"go": "go@1.23"}
		task := &domain.Task{
			Name:       domain.NewInternedString("task1"),
			WorkingDir: domain.NewInternedString("Root"),
			Tools:      tools,
		}
		_ = g.AddTask(task)

		a := app.New(mockLoader, mockExecutor, mockLogger, mockStore, mockHasher, mockResolver, mockEnvFactory).
			WithTeaOptions(
				tea.WithInput(strings.NewReader("")),
				tea.WithOutput(io.Discard),
				tea.WithoutSignalHandler(),
				tea.WithoutRenderer(),
			).
			WithDisableTick()

		mockResolver.EXPECT().ResolveInputs(gomock.Any(), ".").Return([]string{}, nil).AnyTimes()
		mockLoader.EXPECT().Load(".").Return(g, nil)
		mockHasher.EXPECT().ComputeInputHash(task, nil, []string{}).Return("hash", nil)
		mockStore.EXPECT().Get("task1").Return(nil, nil)
		mockEnvFactory.EXPECT().GetEnvironment(gomock.Any(), tools).Return([]string{"PATH=/nix/store/go/bin"}, nil)
		mockExecutor.EXPECT().Execute(
			gomock.Any(), task, []string{"PATH=/nix/store/go/bin"}, gomock.Any(), gomock.Any(),
		).Return(nil)
		mockStore.EXPECT().Put(gomock.Any()).Return(nil)

		err = a.Run(context.Background(), []string{"task1"}, app.RunOptions{NoCache: false})
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})
}

func TestApp_SetLogJSON(t *testing.T) {
	tests := []struct {
		name   string
		enable bool
	}{
		{"Enable JSON logging", true},
		{"Disable JSON logging", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockLogger := mocks.NewMockLogger(ctrl)
			mockLogger.EXPECT().SetJSON(tt.enable).Times(1)

			a := app.New(nil, nil, mockLogger, nil, nil, nil, nil)
			a.SetLogJSON(tt.enable)
		})
	}
}

func TestApp_Clean_RemoveAllError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping permission-based test on Windows")
	}

	synctest.Test(t, func(t *testing.T) {
		cwd, err := os.Getwd()
		require.NoError(t, err)
		defer func() { _ = os.Chdir(cwd) }()

		tmpDir := t.TempDir()
		require.NoError(t, os.Chdir(tmpDir))

		storePath := domain.DefaultStorePath()
		require.NoError(t, os.MkdirAll(storePath, domain.DirPerm))

		childFile := filepath.Join(storePath, "marker.txt")
		require.NoError(t, os.WriteFile(childFile, []byte("test"), domain.FilePerm))
		//nolint:gosec // Intentionally setting restrictive permissions to test error handling
		require.NoError(t, os.Chmod(storePath, 0o555))
		defer func() { _ = os.Chmod(storePath, domain.DirPerm) }()

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockLogger := mocks.NewMockLogger(ctrl)

		a := app.New(nil, nil, mockLogger, nil, nil, nil, nil)
		err = a.Clean(context.Background(), app.CleanOptions{Build: true})

		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to remove build info store")
	})
}
