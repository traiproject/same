package scheduler_test

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"

	"go.trai.ch/bob/internal/core/domain"
	"go.trai.ch/bob/internal/core/ports/mocks"
	"go.trai.ch/bob/internal/engine/scheduler"
	"go.uber.org/mock/gomock"
)

func TestScheduler_Init(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Setup gomock controller
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// Setup
		g := domain.NewGraph()
		task1 := &domain.Task{Name: domain.NewInternedString("task1")}
		task2 := &domain.Task{Name: domain.NewInternedString("task2")}

		if err := g.AddTask(task1); err != nil {
			t.Fatalf("failed to add task1: %v", err)
		}
		if err := g.AddTask(task2); err != nil {
			t.Fatalf("failed to add task2: %v", err)
		}

		mockExec := mocks.NewMockExecutor(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)
		s, err := scheduler.NewScheduler(g, mockExec, mockHasher, mockStore)
		if err != nil {
			t.Fatalf("failed to create scheduler: %v", err)
		}

		// Verify
		taskStatus := s.GetTaskStatusMap()
		if len(taskStatus) != 2 {
			t.Errorf("expected 2 tasks, got %d", len(taskStatus))
		}

		if status, ok := taskStatus[task1.Name]; !ok || status != scheduler.StatusPending {
			t.Errorf("expected task1 to be Pending, got %s", status)
		}
		if status, ok := taskStatus[task2.Name]; !ok || status != scheduler.StatusPending {
			t.Errorf("expected task2 to be Pending, got %s", status)
		}
	})
}

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
		mockHasher := mocks.NewMockHasher(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)

		// Default expectations for Hasher and Store (Cache Miss)
		mockHasher.EXPECT().ComputeInputHash(gomock.Any(), gomock.Any(), gomock.Any()).Return("hash", nil).AnyTimes()
		mockStore.EXPECT().Get(gomock.Any()).Return(nil, nil).AnyTimes()
		mockHasher.EXPECT().ComputeFileHash(gomock.Any()).Return(uint64(123), nil).AnyTimes()
		mockStore.EXPECT().Put(gomock.Any()).Return(nil).AnyTimes()

		s, err := scheduler.NewScheduler(g, mockExec, mockHasher, mockStore)
		if err != nil {
			t.Fatalf("failed to create scheduler: %v", err)
		}

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
			errCh <- s.Run(context.Background(), 2)
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
		err = <-errCh

		// Verify error
		if err == nil {
			t.Error("expected error from Run, got nil")
		}
	})
}

func TestScheduler_Run_CacheHit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// Setup: Single task graph
		g := domain.NewGraph()
		task1 := &domain.Task{
			Name: domain.NewInternedString("task1"),
		}
		_ = g.AddTask(task1)

		mockExec := mocks.NewMockExecutor(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)

		// Setup cache hit scenario
		// ComputeInputHash returns "hash123"
		mockHasher.EXPECT().ComputeInputHash(gomock.Any(), gomock.Any(), gomock.Any()).Return("hash123", nil)

		// Store.Get returns a BuildInfo with matching InputHash
		cachedBuildInfo := &domain.BuildInfo{
			TaskName:  "task1",
			InputHash: "hash123",
		}
		mockStore.EXPECT().Get("task1").Return(cachedBuildInfo, nil)

		// Executor.Execute should NOT be called on cache hit
		mockExec.EXPECT().Execute(gomock.Any(), gomock.Any()).Times(0)

		s, err := scheduler.NewScheduler(g, mockExec, mockHasher, mockStore)
		if err != nil {
			t.Fatalf("failed to create scheduler: %v", err)
		}

		// Run scheduler
		err = s.Run(context.Background(), 1)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		// Verify task status is Cached
		taskStatus := s.GetTaskStatusMap()
		if status := taskStatus[task1.Name]; status != scheduler.StatusCached {
			t.Errorf("expected task1 status to be Cached, got %s", status)
		}
	})
}

func TestScheduler_Run_CacheMiss_And_Save(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// Setup: Single task with outputs
		g := domain.NewGraph()
		task1 := &domain.Task{
			Name: domain.NewInternedString("task1"),
			Outputs: []domain.InternedString{
				domain.NewInternedString("output1.txt"),
				domain.NewInternedString("output2.txt"),
			},
		}
		_ = g.AddTask(task1)

		mockExec := mocks.NewMockExecutor(ctrl)
		mockHasher := mocks.NewMockHasher(ctrl)
		mockStore := mocks.NewMockBuildInfoStore(ctrl)

		// Setup cache miss scenario
		// ComputeInputHash returns "hash456"
		mockHasher.EXPECT().ComputeInputHash(gomock.Any(), gomock.Any(), gomock.Any()).Return("hash456", nil)

		// Store.Get returns nil (cache miss)
		mockStore.EXPECT().Get("task1").Return(nil, nil)

		// Executor.Execute should be called exactly once
		mockExec.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(nil).Times(1)

		// ComputeFileHash should be called for each output
		mockHasher.EXPECT().ComputeFileHash("output1.txt").Return(uint64(111), nil)
		mockHasher.EXPECT().ComputeFileHash("output2.txt").Return(uint64(222), nil)

		// Store.Put should be called to save the new build info
		mockStore.EXPECT().Put(gomock.Any()).DoAndReturn(func(info domain.BuildInfo) error {
			// Verify the BuildInfo has the correct values
			if info.TaskName != "task1" {
				t.Errorf("expected TaskName 'task1', got %s", info.TaskName)
			}
			if info.InputHash != "hash456" {
				t.Errorf("expected InputHash 'hash456', got %s", info.InputHash)
			}
			// OutputHash should be a combination of the file hashes
			expectedOutputHash := "6fde"
			if info.OutputHash != expectedOutputHash {
				t.Errorf("expected OutputHash '%s', got %s", expectedOutputHash, info.OutputHash)
			}
			return nil
		})

		s, err := scheduler.NewScheduler(g, mockExec, mockHasher, mockStore)
		if err != nil {
			t.Fatalf("failed to create scheduler: %v", err)
		}

		// Run scheduler
		err = s.Run(context.Background(), 1)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		// Verify task status is Completed
		taskStatus := s.GetTaskStatusMap()
		if status := taskStatus[task1.Name]; status != scheduler.StatusCompleted {
			t.Errorf("expected task1 status to be Completed, got %s", status)
		}
	})
}
