package scheduler

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"

	"go.trai.ch/bob/internal/core/domain"
	"go.trai.ch/bob/internal/core/ports/mocks"
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
		s, err := NewScheduler(g, mockExec)
		if err != nil {
			t.Fatalf("failed to create scheduler: %v", err)
		}

		// Verify
		s.mu.RLock()
		defer s.mu.RUnlock()

		if len(s.taskStatus) != 2 {
			t.Errorf("expected 2 tasks, got %d", len(s.taskStatus))
		}

		if status, ok := s.taskStatus[task1.Name]; !ok || status != StatusPending {
			t.Errorf("expected task1 to be Pending, got %s", status)
		}
		if status, ok := s.taskStatus[task2.Name]; !ok || status != StatusPending {
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
		taskA := &domain.Task{Name: domain.NewInternedString("A"), Dependencies: []domain.InternedString{domain.NewInternedString("B"), domain.NewInternedString("C")}}
		taskB := &domain.Task{Name: domain.NewInternedString("B"), Dependencies: []domain.InternedString{domain.NewInternedString("D")}}
		taskC := &domain.Task{Name: domain.NewInternedString("C"), Dependencies: []domain.InternedString{domain.NewInternedString("D")}}
		taskD := &domain.Task{Name: domain.NewInternedString("D")}

		_ = g.AddTask(taskA)
		_ = g.AddTask(taskB)
		_ = g.AddTask(taskC)
		_ = g.AddTask(taskD)

		mockExec := mocks.NewMockExecutor(ctrl)
		s, err := NewScheduler(g, mockExec)
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
