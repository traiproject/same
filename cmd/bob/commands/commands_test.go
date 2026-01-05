package commands_test

import (
	"context"
	"testing"

	"go.trai.ch/bob/cmd/bob/commands"
	"go.trai.ch/bob/internal/app"
	"go.trai.ch/bob/internal/core/domain"
	"go.trai.ch/bob/internal/core/ports/mocks"
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
	a := app.New(mockLoader, mockExecutor, mockStore, mockHasher, mockResolver, mockEnvFactory)

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
	a := app.New(mockLoader, mockExecutor, mockStore, mockHasher, mockResolver, mockEnvFactory)

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
	a := app.New(mockLoader, mockExecutor, mockStore, mockHasher, mockResolver, mockEnvFactory)

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
