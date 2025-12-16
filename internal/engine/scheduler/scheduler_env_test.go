package scheduler_test

import (
	"context"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"
	"go.trai.ch/bob/internal/core/domain"
	"go.trai.ch/bob/internal/core/ports/mocks"
	"go.trai.ch/bob/internal/engine/scheduler"
	"go.uber.org/mock/gomock"
)

func TestScheduler_Execute_UsesEnvFactory(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockExec := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)
		mockEnvFactory := mocks.NewMockEnvironmentFactory(ctrl)
		// Mock depResolver and pkgManager are not needed for this test as we mock PrepareTask
		// behavior via EnvFactory indirectly
		// Actually prepareTask uses depResolver and pkgManager.
		// Wait, prepareTask is called in executeTask.
		// If tools are present, prepareTask tries to resolve and install them.
		// We need to mock depResolver and pkgManager if we want prepareTask to succeed.
		mockDepResolver := mocks.NewMockDependencyResolver(ctrl)
		mockPkgManager := mocks.NewMockPackageManager(ctrl)

		s := scheduler.NewScheduler(
			mockExec, mockStore, mockHasher, mockResolver, mockLogger,
			mockDepResolver, mockPkgManager, mockEnvFactory,
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

		// 1. prepareTask calls
		mockDepResolver.EXPECT().Resolve(ctx, "go", "1.22.2").Return("commit-hash", "pkgs.go", nil)
		mockPkgManager.EXPECT().Install(ctx, "go", "commit-hash").Return("/nix/store/go-1.22.2", nil)

		// 2. Input Hashing
		mockResolver.EXPECT().ResolveInputs(gomock.Any(), ".").Return([]string{}, nil)
		mockHasher.EXPECT().ComputeInputHash(task, gomock.Any(), gomock.Any()).Return("hash1", nil)

		// 3. Cache Check
		mockStore.EXPECT().Get("build").Return(nil, nil)

		// 4. Env Factory Resolution
		expectedEnv := []string{"GO_VERSION=1.22.2", "PATH=/nix/store/go/bin"}
		mockEnvFactory.EXPECT().GetEnvironment(ctx, task.Tools).Return(expectedEnv, nil)

		// 5. Execution with Env
		mockExec.EXPECT().Execute(ctx, task, expectedEnv).Return(nil)

		// 6. Output Hashing & Store Put
		mockHasher.EXPECT().ComputeOutputHash(gomock.Any(), ".").Return("outHash", nil)
		mockStore.EXPECT().Put(gomock.Any()).Return(nil)

		err := s.Run(ctx, g, []string{"build"}, 1, false)
		require.NoError(t, err)
	})
}
