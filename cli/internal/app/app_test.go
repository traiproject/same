package app_test

import (
	"context"
	"io"
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
