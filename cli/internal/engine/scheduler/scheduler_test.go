package scheduler_test

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/same/internal/core/domain"
	"go.trai.ch/same/internal/core/ports/mocks"
	"go.trai.ch/same/internal/engine/scheduler"
	"go.uber.org/mock/gomock"
)

func TestScheduler_Run_Diamond(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// Graph: A->B, A->C, B->D, C->D
		g := domain.NewGraph()
		g.SetRoot(".")
		taskA := &domain.Task{
			Name: domain.NewInternedString("A"),
			Dependencies: []domain.InternedString{
				domain.NewInternedString("B"),
				domain.NewInternedString("C"),
			},
		}
		taskB := &domain.Task{
			Name: domain.NewInternedString("B"),
			Dependencies: []domain.InternedString{
				domain.NewInternedString("D"),
			},
		}
		taskC := &domain.Task{
			Name: domain.NewInternedString("C"),
			Dependencies: []domain.InternedString{
				domain.NewInternedString("D"),
			},
		}
		taskD := &domain.Task{Name: domain.NewInternedString("D")}

		_ = g.AddTask(taskA)
		_ = g.AddTask(taskB)
		_ = g.AddTask(taskC)
		_ = g.AddTask(taskD)

		mockExec := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockTracer := mocks.NewMockTracer(ctrl)
		mockSpan := mocks.NewMockSpan(ctrl)

		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockResolver, mockTracer, nil)

		// Telemetry Expectations
		mockTracer.EXPECT().EmitPlan(gomock.Any(), gomock.InAnyOrder([]string{"A", "B", "C", "D"}), gomock.Any(), gomock.Any())
		mockTracer.EXPECT().Start(gomock.Any(), "Hydrating Environments").Return(context.Background(), mockSpan)
		mockSpan.EXPECT().End().Times(4) // 1x Hydration, 3x Tasks (D, B, C)
		mockSpan.EXPECT().RecordError(gomock.Any()).Do(func(err error) {
			if err.Error() != "B failed" {
				t.Errorf("expected error 'B failed', got '%v'", err)
			}
		})

		// Tasks D, B, C run. A is skipped due to deps failing.
		mockTracer.EXPECT().Start(gomock.Any(), "D").Return(context.Background(), mockSpan)
		mockTracer.EXPECT().Start(gomock.Any(), "B").Return(context.Background(), mockSpan)
		mockTracer.EXPECT().Start(gomock.Any(), "C").Return(context.Background(), mockSpan)

		// Channels for synchronization
		dStarted := make(chan struct{})
		dProceed := make(chan struct{})
		bStarted := make(chan struct{})
		bProceed := make(chan struct{})
		cStarted := make(chan struct{})
		cProceed := make(chan struct{})

		mockResolver.EXPECT().ResolveInputs(gomock.Any(), ".").Return([]string{}, nil).Times(3)
		mockHasher.EXPECT().ComputeInputHash(gomock.Any(), nil, []string{}).Return("hash", nil).Times(3)
		mockStore.EXPECT().Get(gomock.Any()).Return(nil, nil).Times(3)
		mockStore.EXPECT().Put(gomock.Any()).Return(nil).Times(2)

		mockExec.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, task *domain.Task, _ []string, _, _ io.Writer) error {
				switch task.Name.String() {
				case "D":
					close(dStarted)
					<-dProceed
					return nil
				case "B":
					close(bStarted)
					<-bProceed
					return errors.New("B failed")
				case "C":
					close(cStarted)
					<-cProceed
					return nil
				default:
					return nil
				}
			}).Times(3)

		errCh := make(chan error)
		go func() {
			errCh <- s.Run(context.Background(), g, []string{"all"}, 2, false)
		}()

		synctest.Wait()
		select {
		case <-dStarted:
		default:
			t.Fatal("D did not start")
		}

		close(dProceed)
		synctest.Wait()

		<-bStarted
		<-cStarted

		close(bProceed)
		close(cProceed)

		err := <-errCh
		if err == nil {
			t.Error("expected error from Run, got nil")
		}
	})
}

func TestScheduler_Run_Partial(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		g := domain.NewGraph()
		g.SetRoot(".")
		taskA := &domain.Task{
			Name: domain.NewInternedString("A"),
			Dependencies: []domain.InternedString{
				domain.NewInternedString("B"),
			},
		}
		taskB := &domain.Task{
			Name: domain.NewInternedString("B"),
			Dependencies: []domain.InternedString{
				domain.NewInternedString("C"),
			},
		}
		taskC := &domain.Task{Name: domain.NewInternedString("C")}
		taskD := &domain.Task{Name: domain.NewInternedString("D")}

		_ = g.AddTask(taskA)
		_ = g.AddTask(taskB)
		_ = g.AddTask(taskC)
		_ = g.AddTask(taskD)

		mockExec := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockTracer := mocks.NewMockTracer(ctrl)
		mockSpan := mocks.NewMockSpan(ctrl)

		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockResolver, mockTracer, nil)

		// Expectations
		mockTracer.EXPECT().EmitPlan(gomock.Any(), gomock.InAnyOrder([]string{"A", "B", "C"}), gomock.Any(), gomock.Any())
		mockTracer.EXPECT().Start(gomock.Any(), "Hydrating Environments").Return(context.Background(), mockSpan)

		// Tasks C, B, A run
		mockTracer.EXPECT().Start(gomock.Any(), "C").Return(context.Background(), mockSpan)
		mockTracer.EXPECT().Start(gomock.Any(), "B").Return(context.Background(), mockSpan)
		mockTracer.EXPECT().Start(gomock.Any(), "A").Return(context.Background(), mockSpan)

		mockSpan.EXPECT().End().Times(4)

		mockResolver.EXPECT().ResolveInputs(gomock.Any(), ".").Return([]string{}, nil).Times(3)
		mockHasher.EXPECT().ComputeInputHash(gomock.Any(), nil, []string{}).Return("hash", nil).Times(3)
		mockStore.EXPECT().Get(gomock.Any()).Return(nil, nil).Times(3)
		mockStore.EXPECT().Put(gomock.Any()).Return(nil).Times(3)

		executedTasks := make(map[string]bool)
		var mu sync.Mutex
		mockExec.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, task *domain.Task, _ []string, _, _ io.Writer) error {
				mu.Lock()
				defer mu.Unlock()
				executedTasks[task.Name.String()] = true
				if task.Name.String() == "D" {
					t.Errorf("Task D should not be executed")
				}
				return nil
			}).Times(3)

		err := s.Run(context.Background(), g, []string{"A"}, 1, false)
		if err != nil {
			t.Errorf("Run failed: %v", err)
		}
	})
}

func TestScheduler_Run_ExplicitAll(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		g := domain.NewGraph()
		g.SetRoot(".")
		taskA := &domain.Task{Name: domain.NewInternedString("A")}
		taskB := &domain.Task{Name: domain.NewInternedString("B")}
		taskC := &domain.Task{Name: domain.NewInternedString("C")}

		_ = g.AddTask(taskA)
		_ = g.AddTask(taskB)
		_ = g.AddTask(taskC)

		mockExec := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockTracer := mocks.NewMockTracer(ctrl)
		mockSpan := mocks.NewMockSpan(ctrl)

		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockResolver, mockTracer, nil)

		mockTracer.EXPECT().EmitPlan(gomock.Any(), gomock.InAnyOrder([]string{"A", "B", "C"}), gomock.Any(), gomock.Any())
		mockTracer.EXPECT().Start(gomock.Any(), "Hydrating Environments").Return(context.Background(), mockSpan)

		mockTracer.EXPECT().Start(gomock.Any(), "A").Return(context.Background(), mockSpan)
		mockTracer.EXPECT().Start(gomock.Any(), "B").Return(context.Background(), mockSpan)
		mockTracer.EXPECT().Start(gomock.Any(), "C").Return(context.Background(), mockSpan)
		mockSpan.EXPECT().End().Times(4)

		mockResolver.EXPECT().ResolveInputs(gomock.Any(), ".").Return([]string{}, nil).Times(3)
		mockHasher.EXPECT().ComputeInputHash(gomock.Any(), nil, []string{}).Return("hash", nil).Times(3)
		mockStore.EXPECT().Get(gomock.Any()).Return(nil, nil).Times(3)
		mockStore.EXPECT().Put(gomock.Any()).Return(nil).Times(3)

		mockExec.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(3)

		err := s.Run(context.Background(), g, []string{"all"}, 2, false)
		if err != nil {
			t.Errorf("Run failed: %v", err)
		}
	})
}

func TestScheduler_Run_EmptyTargets(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		g := domain.NewGraph()
		g.SetRoot(".")
		taskA := &domain.Task{Name: domain.NewInternedString("A")}

		_ = g.AddTask(taskA)

		mockExec := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockTracer := mocks.NewMockTracer(ctrl)

		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockResolver, mockTracer, nil)

		mockTracer.EXPECT().EmitPlan(gomock.Any(), []string{}, gomock.Any(), gomock.Any())
		mockSpan := mocks.NewMockSpan(ctrl)
		mockTracer.EXPECT().Start(gomock.Any(), "Hydrating Environments").Return(context.Background(), mockSpan)
		mockSpan.EXPECT().End()

		mockExec.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		err := s.Run(context.Background(), g, []string{}, 2, false)
		if err != nil {
			t.Errorf("Run failed: %v", err)
		}
	})
}

func TestScheduler_Run_TaskNotFound(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		g := domain.NewGraph()
		g.SetRoot(".")
		mockExec := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockTracer := mocks.NewMockTracer(ctrl)

		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockResolver, mockTracer, nil)

		// Expect failure before EmitPlan happens technically, OR depending on implementation.
		// Current impl: Validate -> newRunState (resolves tasks) -> EmitPlan.
		// ResolveTargetTasks returns error if task not found. So EmitPlan NOT called.

		err := s.Run(context.Background(), g, []string{"B"}, 1, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "task not found")
	})
}

func TestScheduler_CheckTaskCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExec := mocks.NewMockExecutor(ctrl)
	mockStore := mocks.NewMockBuildInfoStore(ctrl)
	mockHasher := mocks.NewMockHasher(ctrl)
	mockResolver := mocks.NewMockInputResolver(ctrl)
	mockTracer := mocks.NewMockTracer(ctrl)

	s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockResolver, mockTracer, nil)
	task := &domain.Task{
		Name:    domain.NewInternedString("test-task"),
		Outputs: []domain.InternedString{domain.NewInternedString("out.txt")},
	}
	ctx := context.Background()
	const testHash = "hash123"
	const outputHash = "outHash123"

	// Case 1: Cache Hit (Hashes match, outputs exist)
	t.Run("CacheHit", func(t *testing.T) {
		mockResolver.EXPECT().ResolveInputs([]string{}, ".").Return([]string{}, nil)
		mockHasher.EXPECT().ComputeInputHash(task, task.Environment, []string{}).Return(testHash, nil)
		mockStore.EXPECT().Get("test-task").Return(&domain.BuildInfo{
			TaskName:   "test-task",
			InputHash:  testHash,
			OutputHash: outputHash,
		}, nil)
		mockHasher.EXPECT().ComputeOutputHash([]string{"out.txt"}, ".").Return(outputHash, nil)

		skipped, h, err := s.CheckTaskCache(ctx, task, ".")
		require.NoError(t, err)
		assert.True(t, skipped)
		assert.Equal(t, testHash, h)
	})

	// Case 2: Cache Miss (Input Hashes mismatch)
	t.Run("CacheMiss_InputMismatch", func(t *testing.T) {
		mockResolver.EXPECT().ResolveInputs([]string{}, ".").Return([]string{}, nil)
		mockHasher.EXPECT().ComputeInputHash(task, task.Environment, []string{}).Return(testHash, nil)
		mockStore.EXPECT().Get("test-task").Return(&domain.BuildInfo{
			TaskName:  "test-task",
			InputHash: "old-hash",
		}, nil)

		skipped, h, err := s.CheckTaskCache(ctx, task, ".")
		require.NoError(t, err)
		assert.False(t, skipped)
		assert.Equal(t, testHash, h)
	})
}

func TestScheduler_Run_Caching(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockExec := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockTracer := mocks.NewMockTracer(ctrl)
		mockSpan := mocks.NewMockSpan(ctrl)

		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockResolver, mockTracer, nil)
		g := domain.NewGraph()
		g.SetRoot(".")
		task := &domain.Task{
			Name:    domain.NewInternedString("build"),
			Outputs: []domain.InternedString{domain.NewInternedString("out")},
		}
		_ = g.AddTask(task)

		ctx := context.Background()
		const hash1 = "hash1"
		const outputHash = "outHash"

		// 1. First Run: Cache Miss
		mockTracer.EXPECT().EmitPlan(gomock.Any(), []string{"build"}, gomock.Any(), gomock.Any())
		mockTracer.EXPECT().Start(gomock.Any(), "Hydrating Environments").Return(ctx, mockSpan)
		mockTracer.EXPECT().Start(gomock.Any(), "build").Return(ctx, mockSpan)
		mockSpan.EXPECT().End().Times(2)

		mockResolver.EXPECT().ResolveInputs([]string{}, ".").Return([]string{}, nil)
		mockHasher.EXPECT().ComputeInputHash(task, task.Environment, []string{}).Return(hash1, nil)
		mockStore.EXPECT().Get("build").Return(nil, nil)
		mockExec.EXPECT().Execute(ctx, task, gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		mockHasher.EXPECT().ComputeOutputHash([]string{"out"}, ".").Return(outputHash, nil)
		mockStore.EXPECT().Put(gomock.Any()).Return(nil)

		err := s.Run(ctx, g, []string{"build"}, 1, false)
		require.NoError(t, err)

		// 2. Second Run: Cache Hit
		mockTracer.EXPECT().EmitPlan(gomock.Any(), []string{"build"}, gomock.Any(), gomock.Any())
		mockTracer.EXPECT().Start(gomock.Any(), "Hydrating Environments").Return(ctx, mockSpan)
		mockTracer.EXPECT().Start(gomock.Any(), "build").Return(ctx, mockSpan)
		mockSpan.EXPECT().End().Times(2)
		mockSpan.EXPECT().SetAttribute("same.cached", true)

		mockResolver.EXPECT().ResolveInputs([]string{}, ".").Return([]string{}, nil)
		mockHasher.EXPECT().ComputeInputHash(task, task.Environment, []string{}).Return(hash1, nil)
		mockStore.EXPECT().Get("build").Return(&domain.BuildInfo{
			TaskName:   "build",
			InputHash:  hash1,
			OutputHash: outputHash,
		}, nil)
		mockHasher.EXPECT().ComputeOutputHash([]string{"out"}, ".").Return(outputHash, nil)

		err = s.Run(ctx, g, []string{"build"}, 1, false)
		require.NoError(t, err)
	})
}
