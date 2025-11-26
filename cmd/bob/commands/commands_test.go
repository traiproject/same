package commands_test

import (
	"context"
	"testing"

	"go.trai.ch/bob/cmd/bob/commands"
	"go.trai.ch/bob/internal/app"
	"go.trai.ch/bob/internal/core/domain"
	"go.trai.ch/bob/internal/core/ports/mocks"
	"go.trai.ch/bob/internal/engine/scheduler"
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
	mockVerifier := mocks.NewMockVerifier(ctrl)
	mockLogger := mocks.NewMockLogger(ctrl)

	// Create a graph with one task named "build"
	g := domain.NewGraph()
	buildTask := &domain.Task{Name: domain.NewInternedString("build")}
	_ = g.AddTask(buildTask)

	// Setup scheduler and app
	sched := scheduler.NewScheduler(mockExecutor, mockStore, mockHasher, mockVerifier, mockLogger)
	a := app.New(mockLoader, sched)

	// Initialize CLI
	cli := commands.New(a)

	// Setup expectations
	mockLoader.EXPECT().Load(".").Return(g, nil)
	mockExecutor.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(nil)

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
	mockVerifier := mocks.NewMockVerifier(ctrl)
	mockLogger := mocks.NewMockLogger(ctrl)

	// Setup scheduler and app
	sched := scheduler.NewScheduler(mockExecutor, mockStore, mockHasher, mockVerifier, mockLogger)
	a := app.New(mockLoader, sched)

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
	mockVerifier := mocks.NewMockVerifier(ctrl)
	mockLogger := mocks.NewMockLogger(ctrl)

	// Setup scheduler and app
	sched := scheduler.NewScheduler(mockExecutor, mockStore, mockHasher, mockVerifier, mockLogger)
	a := app.New(mockLoader, sched)

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
