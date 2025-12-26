package scheduler_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/bob/internal/core/domain"
	"go.trai.ch/bob/internal/core/ports/mocks"
	"go.trai.ch/bob/internal/engine/scheduler"
	"go.uber.org/mock/gomock"
)

func TestScheduler_Run_Diamond(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// Graph: A->B, A->C, B->D, C->D
		// Dependencies:
		// A depends on B, C
		// B depends on D
		// C depends on D
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
		mockLogger := mocks.NewMockLogger(ctrl)
		mockTelemetry := mocks.NewMockTelemetry(ctrl)
		mockVertex := mocks.NewMockVertex(ctrl)
		mockVertex.EXPECT().Complete(gomock.Any()).AnyTimes()
		mockVertex.EXPECT().Cached().AnyTimes()
		mockVertex.EXPECT().Log(gomock.Any(), gomock.Any()).AnyTimes()
		mockTelemetry.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(context.Background(), mockVertex).AnyTimes()
		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockResolver, mockLogger, nil, mockTelemetry)

		// Channels for synchronization
		dStarted := make(chan struct{})
		dProceed := make(chan struct{})
		bStarted := make(chan struct{})
		bProceed := make(chan struct{})
		cStarted := make(chan struct{})
		cProceed := make(chan struct{})

		// Mock Expectations
		// D runs first
		mockResolver.EXPECT().ResolveInputs(gomock.Any(), ".").Return([]string{}, nil).Times(3)
		mockHasher.EXPECT().ComputeInputHash(gomock.Any(), nil, []string{}).Return("hash", nil).Times(3)
		mockStore.EXPECT().Get(gomock.Any()).Return(nil, nil).Times(3)
		mockStore.EXPECT().Put(gomock.Any()).Return(nil).Times(2)

		mockExec.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, task *domain.Task, _ []string) error {
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
				case "A":
					t.Error("Task A should not be executed")
					return nil
				default:
					t.Errorf("Unexpected task: %s", task.Name)
					return nil
				}
			}).Times(3)

		// Run Scheduler in a goroutine
		errCh := make(chan error)
		go func() {
			errCh <- s.Run(context.Background(), g, []string{"all"}, 2, false)
		}()

		// Assert D is running
		synctest.Wait()
		select {
		case <-dStarted:
			// OK
		default:
			t.Fatal("D did not start")
		}

		// Unblock D
		close(dProceed)

		// Assert B and C are running
		synctest.Wait()

		// Wait for both B and C to start
		// Since we are in synctest, we can just receive from channels.
		// If they haven't started, we will deadlock/block, which synctest might detect or we just timeout in real world.
		// But in synctest, if we block on channel, other goroutines run.
		<-bStarted
		<-cStarted

		// Fail B, Finish C
		close(bProceed)
		close(cProceed)

		// Wait for Run to finish
		err := <-errCh

		// Verify error
		if err == nil {
			t.Error("expected error from Run, got nil")
		}
	})
}

func TestScheduler_Run_Partial(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// Graph: A->B, B->C, D
		// Target: A
		// Expected: A, B, C run. D does not run.
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
		mockLogger := mocks.NewMockLogger(ctrl)
		mockTelemetry := mocks.NewMockTelemetry(ctrl)
		mockVertex := mocks.NewMockVertex(ctrl)
		mockVertex.EXPECT().Complete(gomock.Any()).AnyTimes()
		mockVertex.EXPECT().Cached().AnyTimes()
		mockVertex.EXPECT().Log(gomock.Any(), gomock.Any()).AnyTimes()
		mockTelemetry.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(context.Background(), mockVertex).AnyTimes()
		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockResolver, mockLogger, nil, mockTelemetry)

		// Mock Expectations
		mockResolver.EXPECT().ResolveInputs(gomock.Any(), ".").Return([]string{}, nil).Times(3)
		mockHasher.EXPECT().ComputeInputHash(gomock.Any(), nil, []string{}).Return("hash", nil).Times(3)
		mockStore.EXPECT().Get(gomock.Any()).Return(nil, nil).Times(3)
		mockStore.EXPECT().Put(gomock.Any()).Return(nil).Times(3)

		executedTasks := make(map[string]bool)
		var mu sync.Mutex
		mockExec.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, task *domain.Task, _ []string) error {
				mu.Lock()
				defer mu.Unlock()
				executedTasks[task.Name.String()] = true
				if task.Name.String() == "D" {
					t.Errorf("Task D should not be executed")
				}
				return nil
			}).Times(3) // A, B, C

		err := s.Run(context.Background(), g, []string{"A"}, 1, false)
		if err != nil {
			t.Errorf("Run failed: %v", err)
		}

		if !executedTasks["A"] || !executedTasks["B"] || !executedTasks["C"] {
			t.Errorf("Expected A, B, and C to execute, got: %v", executedTasks)
		}
	})
}

func TestScheduler_Run_ExplicitAll(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// Graph: A, B, C (no dependencies)
		// Target: "all"
		// Expected: All tasks run
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
		mockLogger := mocks.NewMockLogger(ctrl)
		mockTelemetry := mocks.NewMockTelemetry(ctrl)
		mockVertex := mocks.NewMockVertex(ctrl)
		mockVertex.EXPECT().Complete(gomock.Any()).AnyTimes()
		mockVertex.EXPECT().Cached().AnyTimes()
		mockVertex.EXPECT().Log(gomock.Any(), gomock.Any()).AnyTimes()
		mockTelemetry.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(context.Background(), mockVertex).AnyTimes()
		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockResolver, mockLogger, nil, mockTelemetry)

		// Expect all three tasks to execute
		mockResolver.EXPECT().ResolveInputs(gomock.Any(), ".").Return([]string{}, nil).Times(3)
		mockHasher.EXPECT().ComputeInputHash(gomock.Any(), nil, []string{}).Return("hash", nil).Times(3)
		mockStore.EXPECT().Get(gomock.Any()).Return(nil, nil).Times(3)
		mockStore.EXPECT().Put(gomock.Any()).Return(nil).Times(3)

		executedTasks := make(map[string]bool)
		var mu sync.Mutex
		mockExec.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, task *domain.Task, _ []string) error {
				mu.Lock()
				defer mu.Unlock()
				executedTasks[task.Name.String()] = true
				return nil
			}).Times(3)

		err := s.Run(context.Background(), g, []string{"all"}, 2, false)
		if err != nil {
			t.Errorf("Run failed: %v", err)
		}

		// Verify all tasks were executed
		if !executedTasks["A"] || !executedTasks["B"] || !executedTasks["C"] {
			t.Errorf("Not all tasks were executed: %v", executedTasks)
		}
	})
}

func TestScheduler_Run_AllWithOtherTargets(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// Graph: A, B, C (no dependencies)
		// Target: ["all", "A"]
		// Expected: All tasks run ("all" takes precedence)
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
		mockLogger := mocks.NewMockLogger(ctrl)
		mockTelemetry := mocks.NewMockTelemetry(ctrl)
		mockVertex := mocks.NewMockVertex(ctrl)
		mockVertex.EXPECT().Complete(gomock.Any()).AnyTimes()
		mockVertex.EXPECT().Cached().AnyTimes()
		mockVertex.EXPECT().Log(gomock.Any(), gomock.Any()).AnyTimes()
		mockTelemetry.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(context.Background(), mockVertex).AnyTimes()
		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockResolver, mockLogger, nil, mockTelemetry)

		// Expect all three tasks to execute
		mockResolver.EXPECT().ResolveInputs(gomock.Any(), ".").Return([]string{}, nil).Times(3)
		mockHasher.EXPECT().ComputeInputHash(gomock.Any(), nil, []string{}).Return("hash", nil).Times(3)
		mockStore.EXPECT().Get(gomock.Any()).Return(nil, nil).Times(3)
		mockStore.EXPECT().Put(gomock.Any()).Return(nil).Times(3)

		executedTasks := make(map[string]bool)
		var mu sync.Mutex
		mockExec.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, task *domain.Task, _ []string) error {
				mu.Lock()
				defer mu.Unlock()
				executedTasks[task.Name.String()] = true
				return nil
			}).Times(3)

		err := s.Run(context.Background(), g, []string{"all", "A"}, 2, false)
		if err != nil {
			t.Errorf("Run failed: %v", err)
		}

		// Verify all tasks were executed
		if !executedTasks["A"] || !executedTasks["B"] || !executedTasks["C"] {
			t.Errorf("Not all tasks were executed: %v", executedTasks)
		}
	})
}

func TestScheduler_Run_EmptyTargets(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// Graph: A, B, C (no dependencies)
		// Target: [] (empty)
		// Expected: No tasks run
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
		mockLogger := mocks.NewMockLogger(ctrl)
		mockTelemetry := mocks.NewMockTelemetry(ctrl)
		mockVertex := mocks.NewMockVertex(ctrl)
		mockVertex.EXPECT().Complete(gomock.Any()).AnyTimes()
		mockVertex.EXPECT().Cached().AnyTimes()
		mockVertex.EXPECT().Log(gomock.Any(), gomock.Any()).AnyTimes()
		mockTelemetry.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(context.Background(), mockVertex).AnyTimes()
		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockResolver, mockLogger, nil, mockTelemetry)

		// Expect no tasks to execute
		mockExec.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		err := s.Run(context.Background(), g, []string{}, 2, false)
		if err != nil {
			t.Errorf("Run failed: %v", err)
		}
	})
}

func TestScheduler_Run_SpecificTargets(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// Graph: A, B, C (no dependencies)
		// Target: ["A", "B"]
		// Expected: Only A and B run, not C
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
		mockLogger := mocks.NewMockLogger(ctrl)
		mockTelemetry := mocks.NewMockTelemetry(ctrl)
		mockVertex := mocks.NewMockVertex(ctrl)
		mockVertex.EXPECT().Complete(gomock.Any()).AnyTimes()
		mockVertex.EXPECT().Cached().AnyTimes()
		mockVertex.EXPECT().Log(gomock.Any(), gomock.Any()).AnyTimes()
		mockTelemetry.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(context.Background(), mockVertex).AnyTimes()
		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockResolver, mockLogger, nil, mockTelemetry)

		// Expect only A and B to execute
		mockResolver.EXPECT().ResolveInputs(gomock.Any(), ".").Return([]string{}, nil).Times(2)
		mockHasher.EXPECT().ComputeInputHash(gomock.Any(), nil, []string{}).Return("hash", nil).Times(2)
		mockStore.EXPECT().Get(gomock.Any()).Return(nil, nil).Times(2)
		mockStore.EXPECT().Put(gomock.Any()).Return(nil).Times(2)

		executedTasks := make(map[string]bool)
		var mu sync.Mutex
		mockExec.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, task *domain.Task, _ []string) error {
				if task.Name.String() == "C" {
					t.Errorf("Task C should not execute")
				}
				mu.Lock()
				defer mu.Unlock()
				executedTasks[task.Name.String()] = true
				return nil
			}).Times(2)

		err := s.Run(context.Background(), g, []string{"A", "B"}, 2, false)
		if err != nil {
			t.Errorf("Run failed: %v", err)
		}

		// Verify A and B were executed, C was not
		if !executedTasks["A"] || !executedTasks["B"] {
			t.Errorf("Expected A and B to execute, got: %v", executedTasks)
		}
		if executedTasks["C"] {
			t.Error("Task C should not have executed")
		}
	})
}

func TestScheduler_Run_TaskNotFound(t *testing.T) {
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
		mockLogger := mocks.NewMockLogger(ctrl)
		mockTelemetry := mocks.NewMockTelemetry(ctrl)
		mockVertex := mocks.NewMockVertex(ctrl)
		mockVertex.EXPECT().Complete(gomock.Any()).AnyTimes()
		mockVertex.EXPECT().Cached().AnyTimes()
		mockVertex.EXPECT().Log(gomock.Any(), gomock.Any()).AnyTimes()
		mockTelemetry.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(context.Background(), mockVertex).AnyTimes()
		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockResolver, mockLogger, nil, mockTelemetry)

		// Expect no execution
		mockExec.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

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
	mockLogger := mocks.NewMockLogger(ctrl)
	mockTelemetry := mocks.NewMockTelemetry(ctrl)
	mockVertex := mocks.NewMockVertex(ctrl)
	mockVertex.EXPECT().Complete(gomock.Any()).AnyTimes()
	mockVertex.EXPECT().Cached().AnyTimes()
	mockTelemetry.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(context.Background(), mockVertex).AnyTimes()

	s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockResolver, mockLogger, nil, mockTelemetry)
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

	// Case 3: Cache Miss (No build info)
	t.Run("CacheMiss_NoInfo", func(t *testing.T) {
		mockResolver.EXPECT().ResolveInputs([]string{}, ".").Return([]string{}, nil)
		mockHasher.EXPECT().ComputeInputHash(task, task.Environment, []string{}).Return(testHash, nil)
		mockStore.EXPECT().Get("test-task").Return(nil, nil)

		skipped, h, err := s.CheckTaskCache(ctx, task, ".")
		require.NoError(t, err)
		assert.False(t, skipped)
		assert.Equal(t, testHash, h)
	})

	// Case 4: Dirty Cache (Hashes match, output hash mismatch)
	t.Run("DirtyCache_OutputMismatch", func(t *testing.T) {
		mockResolver.EXPECT().ResolveInputs([]string{}, ".").Return([]string{}, nil)
		mockHasher.EXPECT().ComputeInputHash(task, task.Environment, []string{}).Return(testHash, nil)
		mockStore.EXPECT().Get("test-task").Return(&domain.BuildInfo{
			TaskName:   "test-task",
			InputHash:  testHash,
			OutputHash: outputHash,
		}, nil)
		mockHasher.EXPECT().ComputeOutputHash([]string{"out.txt"}, ".").Return("different-hash", nil)

		skipped, h, err := s.CheckTaskCache(ctx, task, ".")
		require.NoError(t, err)
		assert.False(t, skipped)
		assert.Equal(t, testHash, h)
	})

	// Case 5: Error in Input Hasher
	t.Run("InputHasherError", func(t *testing.T) {
		mockResolver.EXPECT().ResolveInputs([]string{}, ".").Return([]string{}, nil)
		mockHasher.EXPECT().ComputeInputHash(task, task.Environment, []string{}).Return("", errors.New("hasher error"))

		skipped, _, err := s.CheckTaskCache(ctx, task, ".")
		require.Error(t, err)
		assert.False(t, skipped)
	})

	// Case 6: Error in Store
	t.Run("StoreError", func(t *testing.T) {
		mockResolver.EXPECT().ResolveInputs([]string{}, ".").Return([]string{}, nil)
		mockHasher.EXPECT().ComputeInputHash(task, task.Environment, []string{}).Return(testHash, nil)
		mockStore.EXPECT().Get("test-task").Return(nil, errors.New("store error"))

		skipped, h, err := s.CheckTaskCache(ctx, task, ".")
		require.Error(t, err)
		assert.False(t, skipped)
		assert.Equal(t, testHash, h)
	})

	// Case 7: Error in Output Hasher (treated as miss)
	t.Run("OutputHasherError", func(t *testing.T) {
		mockResolver.EXPECT().ResolveInputs([]string{}, ".").Return([]string{}, nil)
		mockHasher.EXPECT().ComputeInputHash(task, task.Environment, []string{}).Return(testHash, nil)
		mockStore.EXPECT().Get("test-task").Return(&domain.BuildInfo{
			TaskName:   "test-task",
			InputHash:  testHash,
			OutputHash: outputHash,
		}, nil)
		mockHasher.EXPECT().ComputeOutputHash([]string{"out.txt"}, ".").Return("", errors.New("file missing"))

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
		mockLogger := mocks.NewMockLogger(ctrl)
		mockTelemetry := mocks.NewMockTelemetry(ctrl)
		mockVertex := mocks.NewMockVertex(ctrl)
		mockVertex.EXPECT().Complete(gomock.Any()).AnyTimes()
		mockVertex.EXPECT().Cached().AnyTimes()
		mockVertex.EXPECT().Log(gomock.Any(), gomock.Any()).AnyTimes()
		mockTelemetry.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(context.Background(), mockVertex).AnyTimes()

		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockResolver, mockLogger, nil, mockTelemetry)
		g := domain.NewGraph()
		g.SetRoot(".")
		task := &domain.Task{
			Name:    domain.NewInternedString("build"),
			Outputs: []domain.InternedString{domain.NewInternedString("out")},
		}
		_ = g.AddTask(task)

		ctx := context.Background()
		const hash1 = "hash1"
		const hash2 = "hash2"
		const outputHash = "outHash"

		// 1. First Run: Cache Miss (Execution)
		// Hasher returns hash1
		mockResolver.EXPECT().ResolveInputs([]string{}, ".").Return([]string{}, nil)
		mockHasher.EXPECT().ComputeInputHash(task, task.Environment, []string{}).Return(hash1, nil)
		// Store returns nil (no info)
		mockStore.EXPECT().Get("build").Return(nil, nil)
		// Executor runs
		mockExec.EXPECT().Execute(ctx, task, gomock.Any()).Return(nil)
		// Output hasher runs after execution
		mockHasher.EXPECT().ComputeOutputHash([]string{"out"}, ".").Return(outputHash, nil)
		// Store updates with hash1 and outputHash
		mockStore.EXPECT().Put(gomock.Any()).DoAndReturn(func(info domain.BuildInfo) error {
			assert.Equal(t, "build", info.TaskName)
			assert.Equal(t, hash1, info.InputHash)
			assert.Equal(t, outputHash, info.OutputHash)
			return nil
		})

		err := s.Run(ctx, g, []string{"build"}, 1, false)
		require.NoError(t, err)

		// 2. Second Run: Cache Hit (Skip)
		// Hasher returns hash1
		mockResolver.EXPECT().ResolveInputs([]string{}, ".").Return([]string{}, nil)
		mockHasher.EXPECT().ComputeInputHash(task, task.Environment, []string{}).Return(hash1, nil)
		// Store returns info with hash1 and outputHash
		mockStore.EXPECT().Get("build").Return(&domain.BuildInfo{
			TaskName:   "build",
			InputHash:  hash1,
			OutputHash: outputHash,
		}, nil)
		// Output hasher checks outputs
		mockHasher.EXPECT().ComputeOutputHash([]string{"out"}, ".").Return(outputHash, nil)
		// Logger logs skip
		mockLogger.EXPECT().Info(gomock.Any()).Do(func(msg string) {
			assert.Contains(t, msg, "Skipping build (cached)")
		})
		// Executor DOES NOT run
		// Store DOES NOT update

		err = s.Run(ctx, g, []string{"build"}, 1, false)
		require.NoError(t, err)

		// 3. Third Run: Input Modified (Execution)
		// Hasher returns hash2
		mockResolver.EXPECT().ResolveInputs([]string{}, ".").Return([]string{}, nil)
		mockHasher.EXPECT().ComputeInputHash(task, task.Environment, []string{}).Return(hash2, nil)
		// Store returns info with hash1 (irrelevant because input hash mismatch)
		mockStore.EXPECT().Get("build").Return(&domain.BuildInfo{
			TaskName:   "build",
			InputHash:  hash1,
			OutputHash: outputHash,
		}, nil)
		// Hash mismatch -> Executor runs
		mockExec.EXPECT().Execute(ctx, task, gomock.Any()).Return(nil)
		// Output hasher runs after execution
		mockHasher.EXPECT().ComputeOutputHash([]string{"out"}, ".").Return(outputHash, nil)
		// Store updates with hash2
		mockStore.EXPECT().Put(gomock.Any()).DoAndReturn(func(info domain.BuildInfo) error {
			assert.Equal(t, "build", info.TaskName)
			assert.Equal(t, hash2, info.InputHash)
			assert.Equal(t, outputHash, info.OutputHash)
			return nil
		})

		err = s.Run(ctx, g, []string{"build"}, 1, false)
		require.NoError(t, err)
	})
}

func TestScheduler_Run_ForceBypassesCache(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockExec := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)
		mockTelemetry := mocks.NewMockTelemetry(ctrl)
		mockVertex := mocks.NewMockVertex(ctrl)
		mockVertex.EXPECT().Complete(gomock.Any()).AnyTimes()
		mockVertex.EXPECT().Cached().AnyTimes()
		mockVertex.EXPECT().Log(gomock.Any(), gomock.Any()).AnyTimes()
		mockVertex.EXPECT().Log(gomock.Any(), gomock.Any()).AnyTimes()
		mockTelemetry.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(context.Background(), mockVertex).AnyTimes()

		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockResolver, mockLogger, nil, mockTelemetry)
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

		// 1. First Run: Cache Miss (Execution)
		// Hasher returns hash1
		mockResolver.EXPECT().ResolveInputs([]string{}, ".").Return([]string{}, nil)
		mockHasher.EXPECT().ComputeInputHash(task, task.Environment, []string{}).Return(hash1, nil)
		// Store returns nil (no info)
		mockStore.EXPECT().Get("build").Return(nil, nil)
		// Executor runs
		mockExec.EXPECT().Execute(ctx, task, gomock.Any()).Return(nil)
		// Output hasher runs after execution
		mockHasher.EXPECT().ComputeOutputHash([]string{"out"}, ".").Return(outputHash, nil)
		// Store updates with hash1
		mockStore.EXPECT().Put(gomock.Any()).DoAndReturn(func(info domain.BuildInfo) error {
			assert.Equal(t, "build", info.TaskName)
			assert.Equal(t, hash1, info.InputHash)
			assert.Equal(t, outputHash, info.OutputHash)
			return nil
		})

		err := s.Run(ctx, g, []string{"build"}, 1, false)
		require.NoError(t, err)

		// 2. Second Run: Cache Hit (Skip) - force=false
		// Hasher returns hash1
		mockResolver.EXPECT().ResolveInputs([]string{}, ".").Return([]string{}, nil)
		mockHasher.EXPECT().ComputeInputHash(task, task.Environment, []string{}).Return(hash1, nil)
		// Store returns info with hash1
		mockStore.EXPECT().Get("build").Return(&domain.BuildInfo{
			TaskName:   "build",
			InputHash:  hash1,
			OutputHash: outputHash,
		}, nil)
		// Output hasher checks outputs
		mockHasher.EXPECT().ComputeOutputHash([]string{"out"}, ".").Return(outputHash, nil)
		// Logger logs skip
		mockLogger.EXPECT().Info(gomock.Any()).Do(func(msg string) {
			assert.Contains(t, msg, "Skipping build (cached)")
		})
		// Executor DOES NOT run
		// Store DOES NOT update

		err = s.Run(ctx, g, []string{"build"}, 1, false)
		require.NoError(t, err)

		// 3. Third Run: Force Bypass Cache (Execution) - force=true
		// Hasher returns hash1 (still same hash, but we force execution)
		mockResolver.EXPECT().ResolveInputs([]string{}, ".").Return([]string{}, nil)
		mockHasher.EXPECT().ComputeInputHash(task, task.Environment, []string{}).Return(hash1, nil)
		// Store.Get is NOT called (cache check bypassed)
		// Output hasher is NOT called for verification (cache check bypassed)
		// Executor runs despite cache being valid
		mockExec.EXPECT().Execute(ctx, task, gomock.Any()).Return(nil)
		// Output hasher runs after execution
		mockHasher.EXPECT().ComputeOutputHash([]string{"out"}, ".").Return(outputHash, nil)
		// Store updates with hash1
		mockStore.EXPECT().Put(gomock.Any()).DoAndReturn(func(info domain.BuildInfo) error {
			assert.Equal(t, "build", info.TaskName)
			assert.Equal(t, hash1, info.InputHash)
			assert.Equal(t, outputHash, info.OutputHash)
			return nil
		})

		err = s.Run(ctx, g, []string{"build"}, 1, true)
		require.NoError(t, err)
	})
}

func TestScheduler_Run_ContextCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockExec := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)
		mockTelemetry := mocks.NewMockTelemetry(ctrl)
		mockVertex := mocks.NewMockVertex(ctrl)
		mockVertex.EXPECT().Complete(gomock.Any()).AnyTimes()
		mockVertex.EXPECT().Cached().AnyTimes()
		mockVertex.EXPECT().Log(gomock.Any(), gomock.Any()).AnyTimes()
		mockTelemetry.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(context.Background(), mockVertex).AnyTimes()

		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockResolver, mockLogger, nil, mockTelemetry)
		g := domain.NewGraph()
		g.SetRoot(".")
		task := &domain.Task{
			Name: domain.NewInternedString("build"),
		}
		_ = g.AddTask(task)

		// Create a cancellable context
		ctx, cancel := context.WithCancel(context.Background())

		// Channels for synchronization
		taskStarted := make(chan struct{})
		taskProceed := make(chan struct{})

		// Mock expectations
		mockResolver.EXPECT().ResolveInputs([]string{}, ".").Return([]string{}, nil)
		mockHasher.EXPECT().ComputeInputHash(task, task.Environment, []string{}).Return("hash1", nil)
		mockStore.EXPECT().Get("build").Return(nil, nil)

		// Executor blocks until we signal
		mockExec.EXPECT().Execute(gomock.Any(), task, gomock.Any()).DoAndReturn(
			func(_ context.Context, _ *domain.Task, _ []string) error {
				close(taskStarted)
				<-taskProceed
				return nil
			})

		// Store.Put will be called since task completes successfully
		mockStore.EXPECT().Put(gomock.Any()).Return(nil).Times(1)

		// Run scheduler in goroutine
		errCh := make(chan error)
		go func() {
			errCh <- s.Run(ctx, g, []string{"build"}, 1, false)
		}()

		// Wait for task to start
		synctest.Wait()
		<-taskStarted

		// Cancel context while task is running
		cancel()

		// Let task complete
		close(taskProceed)

		// Wait for scheduler to finish
		err := <-errCh

		// Should return context.Canceled error
		require.Error(t, err)
		assert.Contains(t, err.Error(), "context canceled")
	})
}

func TestScheduler_Run_ForceModeHasherError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockExec := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)
		mockTelemetry := mocks.NewMockTelemetry(ctrl)
		mockVertex := mocks.NewMockVertex(ctrl)
		mockVertex.EXPECT().Complete(gomock.Any()).AnyTimes()
		mockVertex.EXPECT().Cached().AnyTimes()
		mockVertex.EXPECT().Log(gomock.Any(), gomock.Any()).AnyTimes()
		mockTelemetry.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(context.Background(), mockVertex).AnyTimes()

		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockResolver, mockLogger, nil, mockTelemetry)
		g := domain.NewGraph()
		g.SetRoot(".")
		task := &domain.Task{
			Name: domain.NewInternedString("build"),
		}
		_ = g.AddTask(task)

		ctx := context.Background()

		// In force mode, hasher is called but store.Get is not
		mockResolver.EXPECT().ResolveInputs([]string{}, ".").Return([]string{}, nil)
		mockHasher.EXPECT().ComputeInputHash(task, task.Environment, []string{}).Return("", errors.New("hasher error"))

		// Run with force=true
		err := s.Run(ctx, g, []string{"build"}, 1, true)

		// Should return the hasher error wrapped
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to compute input hash")
		assert.Contains(t, err.Error(), "hasher error")
	})
}

func TestScheduler_Run_StorePutError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockExec := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)

		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockTelemetry := mocks.NewMockTelemetry(ctrl)
		mockVertex := mocks.NewMockVertex(ctrl)
		mockVertex.EXPECT().Complete(gomock.Any()).AnyTimes()
		mockVertex.EXPECT().Cached().AnyTimes()
		mockVertex.EXPECT().Log(gomock.Any(), gomock.Any()).AnyTimes()
		mockTelemetry.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(context.Background(), mockVertex).AnyTimes()

		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockResolver, mockLogger, nil, mockTelemetry)
		g := domain.NewGraph()
		g.SetRoot(".")
		task := &domain.Task{
			Name: domain.NewInternedString("build"),
		}
		_ = g.AddTask(task)

		ctx := context.Background()
		const hash1 = "hash1"
		mockResolver.EXPECT().ResolveInputs([]string{}, ".").Return([]string{}, nil)

		// Mock expectations
		mockHasher.EXPECT().ComputeInputHash(task, task.Environment, []string{}).Return(hash1, nil)
		mockStore.EXPECT().Get("build").Return(nil, nil)
		mockExec.EXPECT().Execute(ctx, task, gomock.Any()).Return(nil)

		// Store.Put fails but build should still succeed
		mockStore.EXPECT().Put(gomock.Any()).Return(errors.New("store error"))

		// Logger should log the error
		mockLogger.EXPECT().Error(gomock.Any()).Do(func(err error) {
			assert.Contains(t, err.Error(), "failed to update build info store")
			assert.Contains(t, err.Error(), "store error")
		})

		// Run should succeed despite store error
		err := s.Run(ctx, g, []string{"build"}, 1, false)
		require.NoError(t, err)
	})
}

func TestScheduler_Run_EnvironmentCacheInvalidation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockExec := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)

		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockTelemetry := mocks.NewMockTelemetry(ctrl)
		mockVertex := mocks.NewMockVertex(ctrl)
		mockVertex.EXPECT().Complete(gomock.Any()).AnyTimes()
		mockVertex.EXPECT().Cached().AnyTimes()
		mockVertex.EXPECT().Log(gomock.Any(), gomock.Any()).AnyTimes()
		mockTelemetry.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(context.Background(), mockVertex).AnyTimes()

		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockResolver, mockLogger, nil, mockTelemetry)
		g := domain.NewGraph()
		g.SetRoot(".")

		// First task with environment
		task1 := &domain.Task{
			Name:    domain.NewInternedString("build"),
			Outputs: []domain.InternedString{domain.NewInternedString("out")},
			Environment: map[string]string{
				"CGO_ENABLED": "0",
				"GOOS":        "linux",
			},
		}
		_ = g.AddTask(task1)

		ctx := context.Background()
		const hash1 = "hash_with_env1"
		const hash2 = "hash_with_env2"

		// 1. First Run: Cache Miss (Execution) with environment
		// Hasher should be called with task1.Environment
		mockResolver.EXPECT().ResolveInputs([]string{}, ".").Return([]string{}, nil)
		mockHasher.EXPECT().ComputeInputHash(task1, task1.Environment, []string{}).Return(hash1, nil)
		mockStore.EXPECT().Get("build").Return(nil, nil)
		mockExec.EXPECT().Execute(ctx, task1, gomock.Any()).Return(nil)
		// Output hasher runs after execution
		mockHasher.EXPECT().ComputeOutputHash([]string{"out"}, ".").Return("outHash", nil)
		// Store updates with hash1
		mockStore.EXPECT().Put(gomock.Any()).DoAndReturn(func(info domain.BuildInfo) error {
			assert.Equal(t, "build", info.TaskName)
			assert.Equal(t, hash1, info.InputHash)
			return nil
		})

		err := s.Run(ctx, g, []string{"build"}, 1, false)
		require.NoError(t, err)

		// 2. Second Run: Cache Hit (Skip) with same environment
		mockResolver.EXPECT().ResolveInputs([]string{}, ".").Return([]string{}, nil)
		mockHasher.EXPECT().ComputeInputHash(task1, task1.Environment, []string{}).Return(hash1, nil)
		mockStore.EXPECT().Get("build").Return(&domain.BuildInfo{
			TaskName:   "build",
			InputHash:  hash1,
			OutputHash: "outHash",
		}, nil)
		mockHasher.EXPECT().ComputeOutputHash([]string{"out"}, ".").Return("outHash", nil)
		mockLogger.EXPECT().Info(gomock.Any()).Do(func(msg string) {
			assert.Contains(t, msg, "Skipping build (cached)")
		})

		err = s.Run(ctx, g, []string{"build"}, 1, false)
		require.NoError(t, err)

		// 3. Third Run: Environment Changed (Cache Miss -> Execution)
		// Update task with different environment
		task2 := &domain.Task{
			Name:    domain.NewInternedString("build"),
			Outputs: []domain.InternedString{domain.NewInternedString("out")},
			Environment: map[string]string{
				"CGO_ENABLED": "1", // Changed from "0"
				"GOOS":        "linux",
			},
		}

		// Clear and re-add task with new environment
		g = domain.NewGraph()
		g.SetRoot(".")
		_ = g.AddTask(task2)

		// Hasher should be called with task2.Environment (different from task1)
		mockResolver.EXPECT().ResolveInputs([]string{}, ".").Return([]string{}, nil)
		mockHasher.EXPECT().ComputeInputHash(task2, task2.Environment, []string{}).Return(hash2, nil)
		// Store returns old hash (from run 1)
		mockStore.EXPECT().Get("build").Return(&domain.BuildInfo{
			TaskName:  "build",
			InputHash: hash1,
		}, nil)
		// Hash mismatch -> Executor runs
		mockExec.EXPECT().Execute(ctx, task2, gomock.Any()).Return(nil)
		// Output hasher runs after execution
		mockHasher.EXPECT().ComputeOutputHash([]string{"out"}, ".").Return("outHash", nil)
		// Store updates with new hash
		mockStore.EXPECT().Put(gomock.Any()).DoAndReturn(func(info domain.BuildInfo) error {
			assert.Equal(t, "build", info.TaskName)
			assert.Equal(t, hash2, info.InputHash)
			return nil
		})

		err = s.Run(ctx, g, []string{"build"}, 1, false)
		require.NoError(t, err)
	})
}

func TestScheduler_Run_ResolverError(t *testing.T) {
	tests := []struct {
		name  string
		force bool
	}{
		{name: "ForceMode", force: true},
		{name: "NormalMode", force: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				mockExec := mocks.NewMockExecutor(ctrl)
				mockStore := mocks.NewMockBuildInfoStore(ctrl)
				mockHasher := mocks.NewMockHasher(ctrl)
				mockResolver := mocks.NewMockInputResolver(ctrl)
				mockLogger := mocks.NewMockLogger(ctrl)
				mockTelemetry := mocks.NewMockTelemetry(ctrl)
				mockVertex := mocks.NewMockVertex(ctrl)
				mockVertex.EXPECT().Complete(gomock.Any()).AnyTimes()
				mockVertex.EXPECT().Cached().AnyTimes()
		mockVertex.EXPECT().Log(gomock.Any(), gomock.Any()).AnyTimes()
				mockTelemetry.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(context.Background(), mockVertex).AnyTimes()

				s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockResolver, mockLogger, nil, mockTelemetry)
				g := domain.NewGraph()
				g.SetRoot(".")
				task := &domain.Task{
					Name: domain.NewInternedString("build"),
				}
				_ = g.AddTask(task)

				ctx := context.Background()

				// Resolver returns error
				mockResolver.EXPECT().ResolveInputs([]string{}, ".").Return(nil, errors.New("resolver error"))

				// Run with specified force mode
				err := s.Run(ctx, g, []string{"build"}, 1, tt.force)

				// Should return the resolver error wrapped
				require.Error(t, err)
				assert.Contains(t, err.Error(), "failed to resolve inputs")
				assert.Contains(t, err.Error(), "resolver error")
			})
		})
	}
}

func TestScheduler_Run_OutputHashComputationError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockExec := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)

		mockTelemetry := mocks.NewMockTelemetry(ctrl)
		mockVertex := mocks.NewMockVertex(ctrl)
		mockVertex.EXPECT().Complete(gomock.Any()).AnyTimes()
		mockVertex.EXPECT().Cached().AnyTimes()
		mockVertex.EXPECT().Log(gomock.Any(), gomock.Any()).AnyTimes()
		mockTelemetry.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(context.Background(), mockVertex).AnyTimes()
		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockResolver, mockLogger, nil, mockTelemetry)
		g := domain.NewGraph()
		g.SetRoot(".")
		task := &domain.Task{
			Name:    domain.NewInternedString("build"),
			Outputs: []domain.InternedString{domain.NewInternedString("out.txt")},
		}
		_ = g.AddTask(task)

		ctx := context.Background()
		const hash1 = "hash1"

		// Mock expectations
		mockResolver.EXPECT().ResolveInputs([]string{}, ".").Return([]string{}, nil)
		mockHasher.EXPECT().ComputeInputHash(task, task.Environment, []string{}).Return(hash1, nil)
		mockStore.EXPECT().Get("build").Return(nil, nil)
		mockExec.EXPECT().Execute(ctx, task, gomock.Any()).Return(nil)

		// Output hasher fails
		mockHasher.EXPECT().ComputeOutputHash([]string{"out.txt"}, ".").Return("", errors.New("hash computation failed"))

		// Logger should log the error
		mockLogger.EXPECT().Error(gomock.Any()).Do(func(err error) {
			assert.Contains(t, err.Error(), "failed to compute output hash")
			assert.Contains(t, err.Error(), "hash computation failed")
		})

		// Run should succeed despite output hash error (it's logged but not fatal)
		err := s.Run(ctx, g, []string{"build"}, 1, false)
		require.NoError(t, err)
	})
}

func TestScheduler_Run_ContextCancelledDuringScheduling(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockExec := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)

		mockTelemetry := mocks.NewMockTelemetry(ctrl)
		mockVertex := mocks.NewMockVertex(ctrl)
		mockVertex.EXPECT().Complete(gomock.Any()).AnyTimes()
		mockVertex.EXPECT().Cached().AnyTimes()
		mockVertex.EXPECT().Log(gomock.Any(), gomock.Any()).AnyTimes()
		mockTelemetry.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(context.Background(), mockVertex).AnyTimes()
		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockResolver, mockLogger, nil, mockTelemetry)
		g := domain.NewGraph()
		g.SetRoot(".")

		// Create two tasks: A depends on B
		taskB := &domain.Task{Name: domain.NewInternedString("B")}
		taskA := &domain.Task{
			Name:         domain.NewInternedString("A"),
			Dependencies: []domain.InternedString{domain.NewInternedString("B")},
		}
		_ = g.AddTask(taskB)
		_ = g.AddTask(taskA)

		// Create a cancellable context
		ctx, cancel := context.WithCancel(context.Background())

		// Channels for synchronization
		taskStarted := make(chan struct{})
		taskProceed := make(chan struct{})

		// Mock expectations for task B
		mockResolver.EXPECT().ResolveInputs([]string{}, ".").Return([]string{}, nil)
		mockHasher.EXPECT().ComputeInputHash(taskB, taskB.Environment, []string{}).Return("hash1", nil)
		mockStore.EXPECT().Get("B").Return(nil, nil)

		// Task B starts, then we cancel context
		mockExec.EXPECT().Execute(gomock.Any(), taskB, gomock.Any()).DoAndReturn(
			func(_ context.Context, _ *domain.Task, _ []string) error {
				close(taskStarted)
				<-taskProceed
				return errors.New("task failed")
			})

		// Run scheduler in goroutine
		errCh := make(chan error)
		go func() {
			errCh <- s.Run(ctx, g, []string{"all"}, 1, false)
		}()

		// Wait for task B to start
		synctest.Wait()
		<-taskStarted

		// Cancel context while task B is running
		cancel()

		// Let task B complete with error
		close(taskProceed)

		// Wait for scheduler to finish
		err := <-errCh

		// Should return both task error and context.Canceled error
		require.Error(t, err)
		assert.Contains(t, err.Error(), "context canceled")
	})
}

func TestScheduler_Run_ContextCancelledAfterCompletion(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockExec := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)

		mockTelemetry := mocks.NewMockTelemetry(ctrl)
		mockVertex := mocks.NewMockVertex(ctrl)
		mockVertex.EXPECT().Complete(gomock.Any()).AnyTimes()
		mockVertex.EXPECT().Cached().AnyTimes()
		mockVertex.EXPECT().Log(gomock.Any(), gomock.Any()).AnyTimes()
		mockTelemetry.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(context.Background(), mockVertex).AnyTimes()
		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockResolver, mockLogger, nil, mockTelemetry)
		g := domain.NewGraph()
		g.SetRoot(".")
		task := &domain.Task{Name: domain.NewInternedString("build")}
		_ = g.AddTask(task)

		// Create a context that's already canceled
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		// Mock expectations - task won't execute because context is canceled
		// No mock expectations needed as scheduler should exit early

		// Run should return context.Canceled error
		err := s.Run(ctx, g, []string{"build"}, 1, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "context canceled")
	})
}

func TestScheduler_Run_UnsafeOutputPath(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockExec := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)

		mockTelemetry := mocks.NewMockTelemetry(ctrl)
		mockVertex := mocks.NewMockVertex(ctrl)
		mockVertex.EXPECT().Complete(gomock.Any()).AnyTimes()
		mockVertex.EXPECT().Cached().AnyTimes()
		mockVertex.EXPECT().Log(gomock.Any(), gomock.Any()).AnyTimes()
		mockTelemetry.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(context.Background(), mockVertex).AnyTimes()
		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockResolver, mockLogger, nil, mockTelemetry)
		g := domain.NewGraph()
		g.SetRoot(".")

		task := &domain.Task{
			Name:    domain.NewInternedString("unsafe-task"),
			Outputs: []domain.InternedString{domain.NewInternedString("../outside")},
		}
		_ = g.AddTask(task)

		// Mock expectations
		mockResolver.EXPECT().ResolveInputs(gomock.Any(), ".").Return([]string{}, nil)
		mockHasher.EXPECT().ComputeInputHash(task, task.Environment, []string{}).Return("hash1", nil)
		mockStore.EXPECT().Get("unsafe-task").Return(nil, nil)

		// Executor should NOT be called
		mockExec.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		err := s.Run(context.Background(), g, []string{"unsafe-task"}, 1, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "output path is outside project root")
	})
}

func TestScheduler_Run_EnvHydrationFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockEnvFactory := mocks.NewMockEnvironmentFactory(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)
		mockTelemetry := mocks.NewMockTelemetry(ctrl)
		mockVertex := mocks.NewMockVertex(ctrl)
		mockTelemetry.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(context.Background(), mockVertex).AnyTimes()

		s := scheduler.NewScheduler(nil, nil, nil, nil, mockLogger, mockEnvFactory, mockTelemetry)

		// Create a graph with a task that uses tools
		g := domain.NewGraph()
		g.SetRoot(".")
		task := &domain.Task{
			Name:  domain.NewInternedString("build"),
			Tools: map[string]string{"go": "go@1.25.4"},
		}
		_ = g.AddTask(task)

		// Mock hydration failure
		mockLogger.EXPECT().Info(gomock.Any()).AnyTimes()
		mockEnvFactory.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Return(nil, errors.New("hydration failed"))
		// We expect logging of this failure potentially, but mostly Run should return error
		// Note: The code wraps error in "failed to hydrate environment"

		err := s.Run(context.Background(), g, []string{"build"}, 1, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "hydration failed")
	})
}

func TestScheduler_ValidateAndCleanOutputs_Security(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// We need to trigger validateAndCleanOutputs, which happens during execution
		// But it's a private method. We can trigger it by running a task.

		mockExec := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)

		mockTelemetry := mocks.NewMockTelemetry(ctrl)
		mockVertex := mocks.NewMockVertex(ctrl)
		mockVertex.EXPECT().Complete(gomock.Any()).AnyTimes()
		mockVertex.EXPECT().Cached().AnyTimes()
		mockVertex.EXPECT().Log(gomock.Any(), gomock.Any()).AnyTimes()
		mockTelemetry.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(context.Background(), mockVertex).AnyTimes()
		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockResolver, mockLogger, nil, mockTelemetry)

		g := domain.NewGraph()
		g.SetRoot(".")

		// Create task with output outside root
		task := &domain.Task{
			Name:    domain.NewInternedString("pwn"),
			Outputs: []domain.InternedString{domain.NewInternedString("../secret")},
		}
		_ = g.AddTask(task)

		// Setup mocks for execution flow
		mockResolver.EXPECT().ResolveInputs(gomock.Any(), gomock.Any()).Return(nil, nil)
		mockHasher.EXPECT().ComputeInputHash(gomock.Any(), gomock.Any(), gomock.Any()).Return("hash", nil)
		mockStore.EXPECT().Get(gomock.Any()).Return(nil, nil)

		// The executor should NOT be called because validation fails before it

		err := s.Run(context.Background(), g, []string{"pwn"}, 1, false)
		// Run might return aggregate error, or if we just look at result
		require.Error(t, err)
		assert.Contains(t, err.Error(), domain.ErrOutputPathOutsideRoot.Error())
	})
}

func TestScheduler_ValidateAndCleanOutputs_RemoveError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		tmpDir := t.TempDir()
		// Create a directory that we can't remove (e.g. by making parent immutable? Hard on linux user)
		// Or creating a directory and putting a file in it that is open?
		// Actually, standard way is to make parent directory read-only.

		protectedDir := filepath.Join(tmpDir, "protected")
		if err := os.Mkdir(protectedDir, 0o700); err != nil {
			t.Fatal(err)
		}

		// Create a file inside
		outputFile := filepath.Join(protectedDir, "out")
		if err := os.WriteFile(outputFile, []byte("data"), 0o600); err != nil {
			t.Fatal(err)
		}

		// Make parent read-only so we can't unlink 'out'
		//nolint:gosec // Need execution permission for directory
		if err := os.Chmod(protectedDir, 0o500); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.Chmod(protectedDir, 0o700) }() //nolint:gosec // Cleanup

		mockExec := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)

		mockTelemetry := mocks.NewMockTelemetry(ctrl)
		mockVertex := mocks.NewMockVertex(ctrl)
		mockVertex.EXPECT().Complete(gomock.Any()).AnyTimes()
		mockVertex.EXPECT().Cached().AnyTimes()
		mockVertex.EXPECT().Log(gomock.Any(), gomock.Any()).AnyTimes()
		mockTelemetry.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(context.Background(), mockVertex).AnyTimes()
		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockResolver, mockLogger, nil, mockTelemetry)

		g := domain.NewGraph()
		g.SetRoot(protectedDir) // Root is the protected dir

		// output is "out" relative to root
		task := &domain.Task{
			Name:    domain.NewInternedString("build"),
			Outputs: []domain.InternedString{domain.NewInternedString(filepath.Join(protectedDir, "out"))},
		}
		_ = g.AddTask(task)

		mockResolver.EXPECT().ResolveInputs(gomock.Any(), gomock.Any()).Return(nil, nil)
		mockHasher.EXPECT().ComputeInputHash(gomock.Any(), gomock.Any(), gomock.Any()).Return("hash", nil)
		mockStore.EXPECT().Get(gomock.Any()).Return(nil, nil)

		err := s.Run(context.Background(), g, []string{"build"}, 1, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to clean output")
	})
}

func TestScheduler_ComputeOutputHash_Failure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockExec := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockResolver := mocks.NewMockInputResolver(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)

		mockTelemetry := mocks.NewMockTelemetry(ctrl)
		mockVertex := mocks.NewMockVertex(ctrl)
		mockVertex.EXPECT().Complete(gomock.Any()).AnyTimes()
		mockVertex.EXPECT().Cached().AnyTimes()
		mockVertex.EXPECT().Log(gomock.Any(), gomock.Any()).AnyTimes()
		mockTelemetry.EXPECT().Record(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(context.Background(), mockVertex).AnyTimes()
		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockResolver, mockLogger, nil, mockTelemetry)

		g := domain.NewGraph()
		g.SetRoot(".")
		task := &domain.Task{
			Name:    domain.NewInternedString("build"),
			Outputs: []domain.InternedString{domain.NewInternedString("out")},
		}
		_ = g.AddTask(task)

		mockResolver.EXPECT().ResolveInputs(gomock.Any(), gomock.Any()).Return(nil, nil)
		mockHasher.EXPECT().ComputeInputHash(gomock.Any(), gomock.Any(), gomock.Any()).Return("hash", nil)
		mockStore.EXPECT().Get(gomock.Any()).Return(nil, nil)

		mockExec.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

		// Mock Output Hash Failure
		mockHasher.EXPECT().ComputeOutputHash(gomock.Any(), gomock.Any()).Return("", errors.New("hash failed"))
		mockLogger.EXPECT().Error(gomock.Any()) // Should log error

		// Store.Put should NOT be called

		err := s.Run(context.Background(), g, []string{"build"}, 1, false)
		require.NoError(t, err) // Task succeeded, cache update failure doesn't fail build
	})
}
