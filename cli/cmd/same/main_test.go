package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"go.trai.ch/same/internal/app"
	"go.trai.ch/same/internal/core/domain"
	"go.trai.ch/same/internal/core/ports/mocks"
	"go.uber.org/mock/gomock"
)

// TestRun_Success verifies that the run function returns 0 when the command succeeds.
func TestRun_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 1. Setup Mocks
	mockLoader := mocks.NewMockConfigLoader(ctrl)
	mockLogger := mocks.NewMockLogger(ctrl)
	// Other mocks needed for App New
	mockExecutor := mocks.NewMockExecutor(ctrl)
	mockStore := mocks.NewMockBuildInfoStore(ctrl)
	mockHasher := mocks.NewMockHasher(ctrl)
	mockResolver := mocks.NewMockInputResolver(ctrl)
	mockEnvFactory := mocks.NewMockEnvironmentFactory(ctrl)

	// 2. Create Real App with Mocks
	application := app.New(
		mockLoader,
		mockExecutor,
		mockLogger,
		mockStore,
		mockHasher,
		mockResolver,
		mockEnvFactory,
	)

	// 3. Define Provider
	provider := func(_ context.Context) (*app.Components, func(), error) {
		return &app.Components{
			App:    application,
			Logger: mockLogger,
		}, func() {}, nil
	}

	// 4. Capture Stderr
	stderr := new(bytes.Buffer)

	// 5. Run with "version" command
	exitCode := run(context.Background(), []string{"version"}, stderr, provider)
	assert.Equal(t, 0, exitCode)
}

// TestRun_InitializationError verifies that run returns 1 when component initialization fails.
func TestRun_InitializationError(t *testing.T) {
	provider := func(_ context.Context) (*app.Components, func(), error) {
		return nil, nil, errors.New("init failed")
	}

	stderr := new(bytes.Buffer)
	exitCode := run(context.Background(), []string{"version"}, stderr, provider)

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, stderr.String(), "Error: init failed")
}

// TestRun_ExecutionError verifies that run returns 1 when the command execution fails.
func TestRun_ExecutionError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLoader := mocks.NewMockConfigLoader(ctrl)
	mockLogger := mocks.NewMockLogger(ctrl)
	// Stub Logger Error
	mockLogger.EXPECT().Error(gomock.Any()).AnyTimes()

	application := app.New(
		mockLoader,
		mocks.NewMockExecutor(ctrl),
		mockLogger,
		mocks.NewMockBuildInfoStore(ctrl),
		mocks.NewMockHasher(ctrl),
		mocks.NewMockInputResolver(ctrl),
		mocks.NewMockEnvironmentFactory(ctrl),
	)

	provider := func(_ context.Context) (*app.Components, func(), error) {
		return &app.Components{
			App:    application,
			Logger: mockLogger,
		}, func() {}, nil
	}

	// Mock Load failing to simulate execution failure
	mockLoader.EXPECT().Load(".").Return(nil, errors.New("load failed"))

	cwd, _ := os.Getwd()
	tmp := t.TempDir()
	_ = os.Chdir(tmp)
	defer func() {
		_ = os.Chdir(cwd)
	}()

	stderr := new(bytes.Buffer)
	// "run" command requires a target.
	exitCode := run(context.Background(), []string{"run", "target"}, stderr, provider, func(a *app.App) {
		// Disable TUI for test
		a.WithTeaOptions(tea.WithInput(nil))
	})

	assert.Equal(t, 1, exitCode)
}

// TestRun_Signal verifies that the context is canceled on signal.
func TestRun_Signal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// We need a provider that blocks until context is done.
	blockCh := make(chan struct{})

	mockLoader := mocks.NewMockConfigLoader(ctrl)
	mockLoader.EXPECT().Load(gomock.Any()).DoAndReturn(func(_ string) (*domain.Graph, error) {
		select {
		case <-blockCh:
			return nil, context.Canceled
		case <-time.After(5 * time.Second):
			return nil, errors.New("timeout in mock")
		}
	})

	mockLogger := mocks.NewMockLogger(ctrl)
	// Allow logging of the error when context is canceled
	mockLogger.EXPECT().Error(gomock.Any()).AnyTimes()

	application := app.New(
		mockLoader,
		mocks.NewMockExecutor(ctrl),
		mockLogger, // Use the logger we just configured
		mocks.NewMockBuildInfoStore(ctrl),
		mocks.NewMockHasher(ctrl),
		mocks.NewMockInputResolver(ctrl),
		mocks.NewMockEnvironmentFactory(ctrl),
	)

	cwd, _ := os.Getwd()
	tmp := t.TempDir()
	_ = os.Chdir(tmp)
	defer func() {
		_ = os.Chdir(cwd)
	}()

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan int)

	go func() {
		errCh <- run(ctx, []string{"run", "target"}, io.Discard, func(context.Context) (*app.Components, func(), error) {
			return &app.Components{App: application, Logger: mockLogger}, func() {}, nil
		})
	}()

	// Wait a bit to ensure run() reaches Load()
	time.Sleep(100 * time.Millisecond)

	cancel()
	close(blockCh)

	select {
	case ret := <-errCh:
		assert.NotEqual(t, 0, ret)
	case <-time.After(2 * time.Second):
		t.Fatal("TestRun_Signal timed out waiting for run() to return")
	}
}
