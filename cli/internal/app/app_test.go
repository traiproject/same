package app_test

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"testing/synctest"

	tea "github.com/charmbracelet/bubbletea"
	"go.trai.ch/same/internal/app"
	"go.trai.ch/same/internal/core/domain"
	"go.trai.ch/same/internal/core/ports"
	"go.trai.ch/same/internal/core/ports/mocks"
	"go.uber.org/mock/gomock"
)

func TestApp_Build(t *testing.T) {
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

		// Setup Graph
		g := domain.NewGraph()
		g.SetRoot(".")
		task := &domain.Task{Name: domain.NewInternedString("task1"), WorkingDir: domain.NewInternedString("Root")}
		_ = g.AddTask(task)

		// Setup App with tea options that work with synctest
		inputReader, inputWriter := io.Pipe()
		a := app.New(mockLoader, mockExecutor, mockLogger, mockStore, mockHasher, mockResolver, mockEnvFactory).
			WithTeaOptions(
				tea.WithInput(inputReader),
				tea.WithOutput(io.Discard),
				tea.WithoutSignalHandler(),
				tea.WithoutRenderer(),
			)

		mockResolver.EXPECT().ResolveInputs(gomock.Any(), ".").Return([]string{}, nil)
		mockLoader.EXPECT().Load(".").Return(g, nil)
		mockHasher.EXPECT().ComputeInputHash(task, nil, []string{}).Return("hash", nil)
		mockStore.EXPECT().Get("task1").Return(nil, nil)
		mockExecutor.EXPECT().Execute(gomock.Any(), task, gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, _ *domain.Task, _ ports.Environment, _ ports.LogWriter, _ ports.Tracer) error {
				// Send quit command to tea program to stop it
				_, _ = inputWriter.Write([]byte("q"))
				return nil
			})
		mockStore.EXPECT().Put(gomock.Any()).Return(nil)

		err = a.Run(context.Background(), []string{"task1"}, app.RunOptions{NoCache: false})
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		_ = inputWriter.Close()
	})
}

func TestApp_Run_NoTargets(t *testing.T) {
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
		a := app.New(mockLoader, mockExecutor, mockLogger, mockStore, mockHasher, mockResolver, mockEnvFactory).
			WithTeaOptions(
				tea.WithInput(strings.NewReader("")),
				tea.WithOutput(io.Discard),
				tea.WithoutSignalHandler(),
			)

		mockLoader.EXPECT().Load(".").Return(domain.NewGraph(), nil)

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
		a := app.New(mockLoader, mockExecutor, mockLogger, mockStore, mockHasher, mockResolver, mockEnvFactory).
			WithTeaOptions(
				tea.WithInput(strings.NewReader("")),
				tea.WithOutput(io.Discard),
				tea.WithoutSignalHandler(),
			)

		mockLoader.EXPECT().Load(".").Return(nil, errors.New("config load error"))

		err = a.Run(context.Background(), []string{"task1"}, app.RunOptions{NoCache: false})
		if err == nil {
			t.Error("Expected error, got nil")
		}
		if !errors.Is(err, errors.New("config load error")) {
			if err.Error() == "" || !strings.Contains(err.Error(), "failed to load configuration") {
				t.Errorf("Expected error to contain 'failed to load configuration', got '%v'", err)
			}
		}
	})
}

func TestApp_Run_BuildExecutionFailed(t *testing.T) {
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

		inputReader, inputWriter := io.Pipe()
		a := app.New(mockLoader, mockExecutor, mockLogger, mockStore, mockHasher, mockResolver, mockEnvFactory).
			WithTeaOptions(
				tea.WithInput(inputReader),
				tea.WithOutput(io.Discard),
				tea.WithoutSignalHandler(),
				tea.WithoutRenderer(),
			)

		mockResolver.EXPECT().ResolveInputs(gomock.Any(), ".").Return([]string{}, nil)
		mockLoader.EXPECT().Load(".").Return(g, nil)
		mockHasher.EXPECT().ComputeInputHash(task, nil, []string{}).Return("hash", nil)
		mockStore.EXPECT().Get("task1").Return(nil, nil)
		mockExecutor.EXPECT().Execute(gomock.Any(), task, gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, _ *domain.Task, _ ports.Environment, _ ports.LogWriter, _ ports.Tracer) error {
				_, _ = inputWriter.Write([]byte("q"))
				return errors.New("command failed")
			})

		err = a.Run(context.Background(), []string{"task1"}, app.RunOptions{NoCache: false})
		if err == nil {
			t.Error("Expected error, got nil")
		}
		if !errors.Is(err, domain.ErrBuildExecutionFailed) {
			t.Errorf("Expected error to wrap ErrBuildExecutionFailed, got: %v", err)
		}

		_ = inputWriter.Close()
	})
}

func TestApp_Run_LogSetupFailure(t *testing.T) {
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

		if writeErr := os.WriteFile(domain.DefaultSamePath(), []byte("conflict"), domain.PrivateFilePerm); writeErr != nil {
			t.Fatalf("Failed to create conflict file: %v", writeErr)
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

		a := app.New(mockLoader, mockExecutor, mockLogger, mockStore, mockHasher, mockResolver, mockEnvFactory).
			WithTeaOptions(
				tea.WithInput(strings.NewReader("")),
				tea.WithOutput(io.Discard),
				tea.WithoutSignalHandler(),
			)

		err = a.Run(context.Background(), []string{"task1"}, app.RunOptions{NoCache: false})
		if err == nil {
			t.Error("Expected error, got nil")
		}

		if !strings.Contains(err.Error(), "failed to create internal directory") {
			t.Errorf("Expected error containing 'failed to create internal directory', got: %v", err)
		}
	})
}
