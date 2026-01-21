package app_test

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/same/internal/app"
	"go.trai.ch/same/internal/core/domain"
	"go.trai.ch/same/internal/core/ports/mocks"
	"go.uber.org/mock/gomock"
)

// TestApp_Wiring verifies that the app is correctly wired together using Graft
// and that the main execution flow interacts with the ports as expected.
// It mocks out the adapters to ensure we are only testing the "glue" code.
// TestApp_Wiring verifies that the app is correctly wired together using Graft
// and that the main execution flow interacts with the ports as expected.
// It mocks out the adapters to ensure we are only testing the "glue" code.
func TestApp_Wiring(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 1. Create Mocks for all essential ports used in App.Run
	mockExecutor := mocks.NewMockExecutor(ctrl)
	mockStore := mocks.NewMockBuildInfoStore(ctrl)
	mockLoader := mocks.NewMockConfigLoader(ctrl)
	mockLogger := mocks.NewMockLogger(ctrl)
	mockHasher := mocks.NewMockHasher(ctrl)
	mockResolver := mocks.NewMockInputResolver(ctrl)
	mockEnvFactory := mocks.NewMockEnvironmentFactory(ctrl)

	// 2. Initialize the App manually using New()
	// Graft does not allow overriding nodes that are already registered via init() hooks.
	// Therefore, for testing, we use constructor injection to wire the app with mocks.
	application := app.New(
		mockLoader,
		mockExecutor,
		mockLogger,
		mockStore,
		mockHasher,
		mockResolver,
		mockEnvFactory,
	)
	require.NotNil(t, application, "app should not be nil")

	// 3. Set up expectations
	// Ideally, we want to simulate a simple build run:
	// Load -> Resolve Inputs -> Hash -> Execute -> Store

	// target name to build
	targetStr := "foo:bar"
	targetName := domain.NewInternedString(targetStr)

	// Mock Graph Loading
	mockGraph := domain.NewGraph()
	task := &domain.Task{
		Name:         targetName,
		Command:      []string{"echo", "hello"},
		Outputs:      domain.NewInternedStrings([]string{"out"}),
		Dependencies: []domain.InternedString{},
	}
	// We manually inject the task into the graph since we are mocking
	_ = mockGraph.AddTask(task)
	// mockGraph.Validate() is internally called by loader typically, but here we return a ready graph.
	// However, the graph needs to be validated for Walk() to work.
	err := mockGraph.Validate()
	require.NoError(t, err)

	// We expect Load to be called once
	mockLoader.EXPECT().Load(".").Return(mockGraph, nil).Times(1)

	// We expect InputResolver and Hasher to be called for the task
	mockResolver.EXPECT().ResolveInputs(gomock.Any(), gomock.Any()).Return([]string{}, nil).AnyTimes()
	mockHasher.EXPECT().ComputeInputHash(gomock.Any(), gomock.Any(), gomock.Any()).Return("mock_hash", nil).AnyTimes()
	mockHasher.EXPECT().ComputeOutputHash(gomock.Any(), gomock.Any()).Return("mock_output_hash", nil).AnyTimes()

	// We expect EnvFactory to be called
	mockEnvFactory.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Return([]string{}, nil).AnyTimes()

	// We expect Executor to be called once for the task
	mockExecutor.EXPECT().Execute(
		gomock.Any(), // context
		gomock.Any(), // task
		gomock.Any(), // env
		gomock.Any(), // stdout
		gomock.Any(), // stderr
	).DoAndReturn(func(_ context.Context, _ *domain.Task, _ []string, _, _ io.Writer) error {
		// Simulate successful execution
		return nil
	}).Times(1)

	// We expect Store to attempt to put the artifact
	mockStore.EXPECT().Put(gomock.Any()).Return(nil).Times(1)

	// Configure App to use a lifeless TUI so it doesn't mess up the test output
	// and doesn't block.
	// NewProgram(..., WithInput(nil), WithOutput(io.Discard))
	application.WithTeaOptions(tea.WithInput(nil), tea.WithOutput(io.Discard))

	// 4. Run the App
	err = application.Run(ctx, []string{targetStr}, app.RunOptions{
		NoCache: true, // Force execution
	})
	assert.NoError(t, err, "App.Run should succeed")
}

func TestApp_Run_ConfigLoadFailure(t *testing.T) {
	setupTestDir(t)
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLoader := mocks.NewMockConfigLoader(ctrl)
	application := app.New(
		mockLoader,
		mocks.NewMockExecutor(ctrl),
		mocks.NewMockLogger(ctrl),
		mocks.NewMockBuildInfoStore(ctrl),
		mocks.NewMockHasher(ctrl),
		mocks.NewMockInputResolver(ctrl),
		mocks.NewMockEnvironmentFactory(ctrl),
	)
	application.WithTeaOptions(tea.WithInput(nil), tea.WithOutput(io.Discard))

	expectedErr := errors.New("config load failed")
	mockLoader.EXPECT().Load(".").Return(nil, expectedErr)

	err := application.Run(ctx, []string{"target"}, app.RunOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load configuration")
	assert.ErrorIs(t, err, expectedErr)
}

func TestApp_Run_ExecutionFailure(t *testing.T) {
	setupTestDir(t)
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExecutor := mocks.NewMockExecutor(ctrl)
	mockLoader := mocks.NewMockConfigLoader(ctrl)
	mockHasher := mocks.NewMockHasher(ctrl)
	mockResolver := mocks.NewMockInputResolver(ctrl)
	mockEnvFactory := mocks.NewMockEnvironmentFactory(ctrl)
	mockStore := mocks.NewMockBuildInfoStore(ctrl)

	application := app.New(
		mockLoader,
		mockExecutor,
		mocks.NewMockLogger(ctrl),
		mockStore,
		mockHasher,
		mockResolver,
		mockEnvFactory,
	)
	application.WithTeaOptions(tea.WithInput(nil), tea.WithOutput(io.Discard))

	// Defines a task that will be executed
	targetStr := "foo:bar"
	targetName := domain.NewInternedString(targetStr)
	mockGraph := domain.NewGraph()
	task := &domain.Task{
		Name:    targetName,
		Command: []string{"fail"},
	}
	_ = mockGraph.AddTask(task)
	require.NoError(t, mockGraph.Validate())

	mockLoader.EXPECT().Load(".").Return(mockGraph, nil)

	// Scheduler dependencies
	mockResolver.EXPECT().ResolveInputs(gomock.Any(), gomock.Any()).Return([]string{}, nil).AnyTimes()
	mockHasher.EXPECT().ComputeInputHash(gomock.Any(), gomock.Any(), gomock.Any()).Return("hash", nil).AnyTimes()
	mockHasher.EXPECT().ComputeOutputHash(gomock.Any(), gomock.Any()).Return("out_hash", nil).AnyTimes()
	mockEnvFactory.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()

	// Execute returns an error
	mockExecutor.EXPECT().Execute(
		gomock.Any(),
		task,
		gomock.Any(),
		gomock.Any(),
		gomock.Any(),
	).Return(errors.New("exec failed"))

	err := application.Run(ctx, []string{targetStr}, app.RunOptions{NoCache: true})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrBuildExecutionFailed)
}

func TestApp_Run_LogSetupFailure(t *testing.T) {
	// Custom setup to create conflict
	tmp := t.TempDir()
	cwd, _ := os.Getwd()
	// Restore cwd when test is done
	defer func() {
		_ = os.Chdir(cwd)
	}()
	require.NoError(t, os.Chdir(tmp))

	// Create .same as a file to cause mkdir to fail
	require.NoError(t, os.WriteFile(domain.DefaultSamePath(), []byte("conflict"), 0o600))

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	application := app.New(
		mocks.NewMockConfigLoader(ctrl),
		mocks.NewMockExecutor(ctrl),
		mocks.NewMockLogger(ctrl),
		mocks.NewMockBuildInfoStore(ctrl),
		mocks.NewMockHasher(ctrl),
		mocks.NewMockInputResolver(ctrl),
		mocks.NewMockEnvironmentFactory(ctrl),
	)
	application.WithTeaOptions(tea.WithInput(nil), tea.WithOutput(io.Discard))

	err := application.Run(context.Background(), []string{"target"}, app.RunOptions{})
	require.Error(t, err)
	// Expect error about creating internal directory
	assert.Contains(t, err.Error(), "failed to create internal directory")
}

func TestApp_Run_TargetValidation(t *testing.T) {
	setupTestDir(t)
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLoader := mocks.NewMockConfigLoader(ctrl)
	application := app.New(
		mockLoader,
		mocks.NewMockExecutor(ctrl),
		mocks.NewMockLogger(ctrl),
		mocks.NewMockBuildInfoStore(ctrl),
		mocks.NewMockHasher(ctrl),
		mocks.NewMockInputResolver(ctrl),
		mocks.NewMockEnvironmentFactory(ctrl),
	)
	application.WithTeaOptions(tea.WithInput(nil), tea.WithOutput(io.Discard))

	// Load succeeds
	mockGraph := domain.NewGraph()
	mockLoader.EXPECT().Load(".").Return(mockGraph, nil)

	err := application.Run(ctx, []string{}, app.RunOptions{})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNoTargetsSpecified)
}

func setupTestDir(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	cwd, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmp))
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})
}
