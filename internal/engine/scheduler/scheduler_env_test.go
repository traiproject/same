package scheduler_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.trai.ch/bob/internal/core/domain"
	"go.trai.ch/bob/internal/core/ports/mocks"
	"go.trai.ch/bob/internal/engine/scheduler"
	"go.uber.org/mock/gomock"
)

func TestScheduler_Execute_UsesEnvFactory(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExec := mocks.NewMockExecutor(ctrl)
	mockStore := mocks.NewMockBuildInfoStore(ctrl)
	mockHasher := mocks.NewMockHasher(ctrl)
	mockResolver := mocks.NewMockInputResolver(ctrl)
	mockTracer := mocks.NewMockTracer(ctrl)
	mockEnvFactory := mocks.NewMockEnvironmentFactory(ctrl)
	mockSpan := mocks.NewMockSpan(ctrl)

	s := scheduler.NewScheduler(
		mockExec, mockStore, mockHasher, mockResolver, mockTracer,
		mockEnvFactory,
	)

	g := domain.NewGraph()
	g.SetRoot(".")
	task := &domain.Task{
		Name: domain.NewInternedString("build"),
		Tools: map[string]string{
			"go": "go@1.22.2",
		},
		Outputs: []domain.InternedString{domain.NewInternedString("out")},
	}
	_ = g.AddTask(task)

	ctx := context.Background()

	// Mock Expectations

	// 1. EmitPlan
	mockTracer.EXPECT().EmitPlan(gomock.Any(), []string{"build"})
	mockTracer.EXPECT().Start(gomock.Any(), "Hydrating Environments").Return(ctx, mockSpan)
	// We expect hydration to be called.
	// 4. Env Factory Resolution
	expectedEnv := []string{"GO_VERSION=1.22.2", "PATH=/nix/store/go/bin"}
	// mockLogger.EXPECT().Info removed
	mockEnvFactory.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Return(expectedEnv, nil)
	mockSpan.EXPECT().End() // Hydration end
	
	// Task Execution
	mockTracer.EXPECT().Start(gomock.Any(), "build").Return(ctx, mockSpan)
	mockSpan.EXPECT().End() // Task end

	// 2. Input Hashing
	mockResolver.EXPECT().ResolveInputs(gomock.Any(), ".").Return([]string{}, nil)
	mockHasher.EXPECT().ComputeInputHash(task, gomock.Any(), gomock.Any()).Return("hash1", nil)

	// 3. Cache Check
	mockStore.EXPECT().Get("build").Return(nil, nil)

	// 5. Execution with Env
	mockExec.EXPECT().Execute(ctx, task, expectedEnv, gomock.Any(), gomock.Any()).Return(nil)

	// 6. Output Hashing & Store Put
	mockHasher.EXPECT().ComputeOutputHash(gomock.Any(), ".").Return("outHash", nil)
	mockStore.EXPECT().Put(gomock.Any()).Return(nil)

	err := s.Run(ctx, g, []string{"build"}, 1, false)
	require.NoError(t, err)
}
