package scheduler_test

import (
	"context"
	"errors"
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
		mockVerifier := mocks.NewMockVerifier(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)
		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockVerifier, mockLogger)

		// Channels for synchronization
		dStarted := make(chan struct{})
		dProceed := make(chan struct{})
		bStarted := make(chan struct{})
		bProceed := make(chan struct{})
		cStarted := make(chan struct{})
		cProceed := make(chan struct{})

		// Mock Expectations
		// D runs first
		mockHasher.EXPECT().ComputeInputHash(gomock.Any(), nil, ".").Return("hash", nil).AnyTimes()
		mockStore.EXPECT().Get(gomock.Any()).Return(nil, nil).AnyTimes()
		mockStore.EXPECT().Put(gomock.Any()).Return(nil).AnyTimes()

		mockExec.EXPECT().Execute(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, task *domain.Task) error {
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
		}).AnyTimes()

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
		mockVerifier := mocks.NewMockVerifier(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)
		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockVerifier, mockLogger)

		// Mock Expectations
		mockHasher.EXPECT().ComputeInputHash(gomock.Any(), nil, ".").Return("hash", nil).AnyTimes()
		mockStore.EXPECT().Get(gomock.Any()).Return(nil, nil).AnyTimes()
		mockStore.EXPECT().Put(gomock.Any()).Return(nil).AnyTimes()

		executedTasks := make(map[string]bool)
		var mu sync.Mutex
		mockExec.EXPECT().Execute(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, task *domain.Task) error {
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
		mockVerifier := mocks.NewMockVerifier(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)
		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockVerifier, mockLogger)

		// Expect all three tasks to execute
		mockHasher.EXPECT().ComputeInputHash(gomock.Any(), nil, ".").Return("hash", nil).AnyTimes()
		mockStore.EXPECT().Get(gomock.Any()).Return(nil, nil).AnyTimes()
		mockStore.EXPECT().Put(gomock.Any()).Return(nil).AnyTimes()

		executedTasks := make(map[string]bool)
		var mu sync.Mutex
		mockExec.EXPECT().Execute(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, task *domain.Task) error {
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
		mockVerifier := mocks.NewMockVerifier(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)
		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockVerifier, mockLogger)

		// Expect all three tasks to execute
		mockHasher.EXPECT().ComputeInputHash(gomock.Any(), nil, ".").Return("hash", nil).AnyTimes()
		mockStore.EXPECT().Get(gomock.Any()).Return(nil, nil).AnyTimes()
		mockStore.EXPECT().Put(gomock.Any()).Return(nil).AnyTimes()

		executedTasks := make(map[string]bool)
		var mu sync.Mutex
		mockExec.EXPECT().Execute(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, task *domain.Task) error {
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
		mockVerifier := mocks.NewMockVerifier(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)
		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockVerifier, mockLogger)

		// Expect no tasks to execute
		mockExec.EXPECT().Execute(gomock.Any(), gomock.Any()).Times(0)

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
		mockVerifier := mocks.NewMockVerifier(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)
		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockVerifier, mockLogger)

		// Expect only A and B to execute
		mockHasher.EXPECT().ComputeInputHash(gomock.Any(), nil, ".").Return("hash", nil).AnyTimes()
		mockStore.EXPECT().Get(gomock.Any()).Return(nil, nil).AnyTimes()
		mockStore.EXPECT().Put(gomock.Any()).Return(nil).AnyTimes()

		executedTasks := make(map[string]bool)
		var mu sync.Mutex
		mockExec.EXPECT().Execute(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, task *domain.Task) error {
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
		mockVerifier := mocks.NewMockVerifier(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)
		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockVerifier, mockLogger)

		// Expect no execution
		mockExec.EXPECT().Execute(gomock.Any(), gomock.Any()).Times(0)

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
	mockVerifier := mocks.NewMockVerifier(ctrl)
	mockLogger := mocks.NewMockLogger(ctrl)

	s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockVerifier, mockLogger)
	task := &domain.Task{
		Name:    domain.NewInternedString("test-task"),
		Outputs: []domain.InternedString{domain.NewInternedString("out.txt")},
	}
	ctx := context.Background()
	const testHash = "hash123"

	// Case 1: Cache Hit (Hashes match, outputs exist)
	t.Run("CacheHit", func(t *testing.T) {
		mockHasher.EXPECT().ComputeInputHash(task, task.Environment, ".").Return(testHash, nil)
		mockStore.EXPECT().Get("test-task").Return(&domain.BuildInfo{
			TaskName:  "test-task",
			InputHash: testHash,
		}, nil)
		mockVerifier.EXPECT().VerifyOutputs(".", []string{"out.txt"}).Return(true, nil)

		skipped, h, err := s.CheckTaskCache(ctx, task, ".")
		require.NoError(t, err)
		assert.True(t, skipped)
		assert.Equal(t, testHash, h)
	})

	// Case 2: Cache Miss (Hashes mismatch)
	t.Run("CacheMiss", func(t *testing.T) {
		mockHasher.EXPECT().ComputeInputHash(task, task.Environment, ".").Return(testHash, nil)
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
		mockHasher.EXPECT().ComputeInputHash(task, task.Environment, ".").Return(testHash, nil)
		mockStore.EXPECT().Get("test-task").Return(nil, nil)

		skipped, h, err := s.CheckTaskCache(ctx, task, ".")
		require.NoError(t, err)
		assert.False(t, skipped)
		assert.Equal(t, testHash, h)
	})

	// Case 4: Dirty Cache (Hashes match, outputs missing)
	t.Run("DirtyCache", func(t *testing.T) {
		mockHasher.EXPECT().ComputeInputHash(task, task.Environment, ".").Return(testHash, nil)
		mockStore.EXPECT().Get("test-task").Return(&domain.BuildInfo{
			TaskName:  "test-task",
			InputHash: testHash,
		}, nil)
		mockVerifier.EXPECT().VerifyOutputs(".", []string{"out.txt"}).Return(false, nil)

		skipped, h, err := s.CheckTaskCache(ctx, task, ".")
		require.NoError(t, err)
		assert.False(t, skipped)
		assert.Equal(t, testHash, h)
	})

	// Case 5: Error in Hasher
	t.Run("HasherError", func(t *testing.T) {
		mockHasher.EXPECT().ComputeInputHash(task, task.Environment, ".").Return("", errors.New("hasher error"))

		skipped, _, err := s.CheckTaskCache(ctx, task, ".")
		require.Error(t, err)
		assert.False(t, skipped)
	})

	// Case 6: Error in Store
	t.Run("StoreError", func(t *testing.T) {
		mockHasher.EXPECT().ComputeInputHash(task, task.Environment, ".").Return(testHash, nil)
		mockStore.EXPECT().Get("test-task").Return(nil, errors.New("store error"))

		skipped, h, err := s.CheckTaskCache(ctx, task, ".")
		require.Error(t, err)
		assert.False(t, skipped)
		assert.Equal(t, testHash, h)
	})

	// Case 7: Error in Verifier
	t.Run("VerifierError", func(t *testing.T) {
		mockHasher.EXPECT().ComputeInputHash(task, task.Environment, ".").Return(testHash, nil)
		mockStore.EXPECT().Get("test-task").Return(&domain.BuildInfo{
			TaskName:  "test-task",
			InputHash: testHash,
		}, nil)
		mockVerifier.EXPECT().VerifyOutputs(".", []string{"out.txt"}).Return(false, errors.New("verifier error"))

		skipped, h, err := s.CheckTaskCache(ctx, task, ".")
		require.Error(t, err)
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
		mockVerifier := mocks.NewMockVerifier(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)

		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockVerifier, mockLogger)
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

		// 1. First Run: Cache Miss (Execution)
		// Hasher returns hash1
		mockHasher.EXPECT().ComputeInputHash(task, task.Environment, ".").Return(hash1, nil)
		// Store returns nil (no info)
		mockStore.EXPECT().Get("build").Return(nil, nil)
		// Executor runs
		mockExec.EXPECT().Execute(ctx, task).Return(nil)
		// Store updates with hash1
		mockStore.EXPECT().Put(gomock.Any()).DoAndReturn(func(info domain.BuildInfo) error {
			assert.Equal(t, "build", info.TaskName)
			assert.Equal(t, hash1, info.InputHash)
			return nil
		})

		err := s.Run(ctx, g, []string{"build"}, 1, false)
		require.NoError(t, err)

		// 2. Second Run: Cache Hit (Skip)
		// Hasher returns hash1
		mockHasher.EXPECT().ComputeInputHash(task, task.Environment, ".").Return(hash1, nil)
		// Store returns info with hash1
		mockStore.EXPECT().Get("build").Return(&domain.BuildInfo{
			TaskName:  "build",
			InputHash: hash1,
		}, nil)
		// Verifier checks outputs
		mockVerifier.EXPECT().VerifyOutputs(".", []string{"out"}).Return(true, nil)
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
		mockHasher.EXPECT().ComputeInputHash(task, task.Environment, ".").Return(hash2, nil)
		// Store returns info with hash1
		mockStore.EXPECT().Get("build").Return(&domain.BuildInfo{
			TaskName:  "build",
			InputHash: hash1,
		}, nil)
		// Executor runs
		mockExec.EXPECT().Execute(ctx, task).Return(nil)
		// Store updates with hash2
		mockStore.EXPECT().Put(gomock.Any()).DoAndReturn(func(info domain.BuildInfo) error {
			assert.Equal(t, "build", info.TaskName)
			assert.Equal(t, hash2, info.InputHash)
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
		mockVerifier := mocks.NewMockVerifier(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)

		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockVerifier, mockLogger)
		g := domain.NewGraph()
		g.SetRoot(".")
		task := &domain.Task{
			Name:    domain.NewInternedString("build"),
			Outputs: []domain.InternedString{domain.NewInternedString("out")},
		}
		_ = g.AddTask(task)

		ctx := context.Background()
		const hash1 = "hash1"

		// 1. First Run: Cache Miss (Execution)
		// Hasher returns hash1
		mockHasher.EXPECT().ComputeInputHash(task, task.Environment, ".").Return(hash1, nil)
		// Store returns nil (no info)
		mockStore.EXPECT().Get("build").Return(nil, nil)
		// Executor runs
		mockExec.EXPECT().Execute(ctx, task).Return(nil)
		// Store updates with hash1
		mockStore.EXPECT().Put(gomock.Any()).DoAndReturn(func(info domain.BuildInfo) error {
			assert.Equal(t, "build", info.TaskName)
			assert.Equal(t, hash1, info.InputHash)
			return nil
		})

		err := s.Run(ctx, g, []string{"build"}, 1, false)
		require.NoError(t, err)

		// 2. Second Run: Cache Hit (Skip) - force=false
		// Hasher returns hash1
		mockHasher.EXPECT().ComputeInputHash(task, task.Environment, ".").Return(hash1, nil)
		// Store returns info with hash1
		mockStore.EXPECT().Get("build").Return(&domain.BuildInfo{
			TaskName:  "build",
			InputHash: hash1,
		}, nil)
		// Verifier checks outputs
		mockVerifier.EXPECT().VerifyOutputs(".", []string{"out"}).Return(true, nil)
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
		mockHasher.EXPECT().ComputeInputHash(task, task.Environment, ".").Return(hash1, nil)
		// Store.Get is NOT called (cache check bypassed)
		// Verifier is NOT called (cache check bypassed)
		// Executor runs despite cache being valid
		mockExec.EXPECT().Execute(ctx, task).Return(nil)
		// Store updates with hash1
		mockStore.EXPECT().Put(gomock.Any()).DoAndReturn(func(info domain.BuildInfo) error {
			assert.Equal(t, "build", info.TaskName)
			assert.Equal(t, hash1, info.InputHash)
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
		mockVerifier := mocks.NewMockVerifier(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)

		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockVerifier, mockLogger)
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
		mockHasher.EXPECT().ComputeInputHash(task, task.Environment, ".").Return("hash1", nil)
		mockStore.EXPECT().Get("build").Return(nil, nil)

		// Executor blocks until we signal
		mockExec.EXPECT().Execute(gomock.Any(), task).DoAndReturn(func(_ context.Context, _ *domain.Task) error {
			close(taskStarted)
			<-taskProceed
			return nil
		})

		// Store.Put will be called since task completes successfully
		mockStore.EXPECT().Put(gomock.Any()).Return(nil).AnyTimes()

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
		mockVerifier := mocks.NewMockVerifier(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)

		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockVerifier, mockLogger)
		g := domain.NewGraph()
		g.SetRoot(".")
		task := &domain.Task{
			Name: domain.NewInternedString("build"),
		}
		_ = g.AddTask(task)

		ctx := context.Background()

		// In force mode, hasher is called but store.Get is not
		mockHasher.EXPECT().ComputeInputHash(task, task.Environment, ".").Return("", errors.New("hasher error"))

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
		mockVerifier := mocks.NewMockVerifier(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)

		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockVerifier, mockLogger)
		g := domain.NewGraph()
		g.SetRoot(".")
		task := &domain.Task{
			Name: domain.NewInternedString("build"),
		}
		_ = g.AddTask(task)

		ctx := context.Background()
		const hash1 = "hash1"

		// Mock expectations
		mockHasher.EXPECT().ComputeInputHash(task, task.Environment, ".").Return(hash1, nil)
		mockStore.EXPECT().Get("build").Return(nil, nil)
		mockExec.EXPECT().Execute(ctx, task).Return(nil)

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
		mockVerifier := mocks.NewMockVerifier(ctrl)
		mockLogger := mocks.NewMockLogger(ctrl)

		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockVerifier, mockLogger)
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
		mockHasher.EXPECT().ComputeInputHash(task1, task1.Environment, ".").Return(hash1, nil)
		mockStore.EXPECT().Get("build").Return(nil, nil)
		mockExec.EXPECT().Execute(ctx, task1).Return(nil)
		mockStore.EXPECT().Put(gomock.Any()).DoAndReturn(func(info domain.BuildInfo) error {
			assert.Equal(t, "build", info.TaskName)
			assert.Equal(t, hash1, info.InputHash)
			return nil
		})

		err := s.Run(ctx, g, []string{"build"}, 1, false)
		require.NoError(t, err)

		// 2. Second Run: Cache Hit (Skip) with same environment
		mockHasher.EXPECT().ComputeInputHash(task1, task1.Environment, ".").Return(hash1, nil)
		mockStore.EXPECT().Get("build").Return(&domain.BuildInfo{
			TaskName:  "build",
			InputHash: hash1,
		}, nil)
		mockVerifier.EXPECT().VerifyOutputs(".", []string{"out"}).Return(true, nil)
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
		mockHasher.EXPECT().ComputeInputHash(task2, task2.Environment, ".").Return(hash2, nil)
		// Store returns old hash (from run 1)
		mockStore.EXPECT().Get("build").Return(&domain.BuildInfo{
			TaskName:  "build",
			InputHash: hash1,
		}, nil)
		// Hash mismatch -> Executor runs
		mockExec.EXPECT().Execute(ctx, task2).Return(nil)
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
