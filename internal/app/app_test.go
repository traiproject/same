package app_test

import (
	"context"
	"testing"
	"testing/synctest"

	"go.trai.ch/bob/internal/app"
	"go.trai.ch/bob/internal/core/domain"
	"go.trai.ch/bob/internal/core/ports/mocks"
	"go.trai.ch/bob/internal/engine/scheduler"
	"go.uber.org/mock/gomock"
)

func TestApp_Build(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockLoader := mocks.NewMockConfigLoader(ctrl)
		mockExecutor := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)

		// Setup Graph
		g := domain.NewGraph()
		task := &domain.Task{Name: domain.NewInternedString("task1")}
		_ = g.AddTask(task)

		// Setup App
		sched := scheduler.NewScheduler(mockExecutor, mockStore, mockHasher)
		a := app.New(mockLoader, sched)

		// Expectations
		mockLoader.EXPECT().Load(".").Return(g, nil)
		mockExecutor.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(nil)

		// Execute
		err := a.Run(context.Background(), []string{"task1"})
		if err != nil {
			t.Errorf("Build failed: %v", err)
		}
	})
}

func TestApp_Run_NoTargets(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockLoader := mocks.NewMockConfigLoader(ctrl)
		mockExecutor := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)

		// Setup App
		sched := scheduler.NewScheduler(mockExecutor, mockStore, mockHasher)
		a := app.New(mockLoader, sched)

		// Expectations
		mockLoader.EXPECT().Load(".").Return(domain.NewGraph(), nil)

		// Execute
		err := a.Run(context.Background(), nil)
		if err == nil {
			t.Error("Expected error, got nil")
		}
		if err.Error() != "no targets specified" {
			t.Errorf("Expected 'no targets specified', got '%v'", err)
		}
	})
}
