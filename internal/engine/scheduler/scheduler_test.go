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
		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher)

		// Channels for synchronization
		dStarted := make(chan struct{})
		dProceed := make(chan struct{})
		bStarted := make(chan struct{})
		bProceed := make(chan struct{})
		cStarted := make(chan struct{})
		cProceed := make(chan struct{})

		// Mock Expectations
		// D runs first
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
			errCh <- s.Run(context.Background(), g, []string{"all"}, 2)
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
		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher)

		// Mock Expectations
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

		err := s.Run(context.Background(), g, []string{"A"}, 1)
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
		taskA := &domain.Task{Name: domain.NewInternedString("A")}
		taskB := &domain.Task{Name: domain.NewInternedString("B")}
		taskC := &domain.Task{Name: domain.NewInternedString("C")}

		_ = g.AddTask(taskA)
		_ = g.AddTask(taskB)
		_ = g.AddTask(taskC)

		mockExec := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher)

		// Expect all three tasks to execute
		executedTasks := make(map[string]bool)
		var mu sync.Mutex
		mockExec.EXPECT().Execute(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, task *domain.Task) error {
			mu.Lock()
			defer mu.Unlock()
			executedTasks[task.Name.String()] = true
			return nil
		}).Times(3)

		err := s.Run(context.Background(), g, []string{"all"}, 2)
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
		taskA := &domain.Task{Name: domain.NewInternedString("A")}
		taskB := &domain.Task{Name: domain.NewInternedString("B")}
		taskC := &domain.Task{Name: domain.NewInternedString("C")}

		_ = g.AddTask(taskA)
		_ = g.AddTask(taskB)
		_ = g.AddTask(taskC)

		mockExec := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher)

		// Expect all three tasks to execute
		executedTasks := make(map[string]bool)
		var mu sync.Mutex
		mockExec.EXPECT().Execute(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, task *domain.Task) error {
			mu.Lock()
			defer mu.Unlock()
			executedTasks[task.Name.String()] = true
			return nil
		}).Times(3)

		err := s.Run(context.Background(), g, []string{"all", "A"}, 2)
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
		taskA := &domain.Task{Name: domain.NewInternedString("A")}
		taskB := &domain.Task{Name: domain.NewInternedString("B")}
		taskC := &domain.Task{Name: domain.NewInternedString("C")}

		_ = g.AddTask(taskA)
		_ = g.AddTask(taskB)
		_ = g.AddTask(taskC)

		mockExec := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher)

		// Expect no tasks to execute
		mockExec.EXPECT().Execute(gomock.Any(), gomock.Any()).Times(0)

		err := s.Run(context.Background(), g, []string{}, 2)
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
		taskA := &domain.Task{Name: domain.NewInternedString("A")}
		taskB := &domain.Task{Name: domain.NewInternedString("B")}
		taskC := &domain.Task{Name: domain.NewInternedString("C")}

		_ = g.AddTask(taskA)
		_ = g.AddTask(taskB)
		_ = g.AddTask(taskC)

		mockExec := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher)

		// Expect only A and B to execute
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

		err := s.Run(context.Background(), g, []string{"A", "B"}, 2)
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
		taskA := &domain.Task{Name: domain.NewInternedString("A")}
		_ = g.AddTask(taskA)

		mockExec := mocks.NewMockExecutor(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		s := scheduler.NewScheduler(mockExec, mockStore, mockHasher)

		// Expect no execution
		mockExec.EXPECT().Execute(gomock.Any(), gomock.Any()).Times(0)

		err := s.Run(context.Background(), g, []string{"B"}, 1)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "task not found")
	})
}
