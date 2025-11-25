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
		s := scheduler.NewScheduler(mockExec)

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
			errCh <- s.Run(context.Background(), g, nil, 2)
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
		// Expected: C, B, A run. D does not run.
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
		s := scheduler.NewScheduler(mockExec)

		// Mock Expectations
		mockExec.EXPECT().Execute(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, task *domain.Task) error {
			if task.Name.String() == "D" {
				t.Error("Task D should not be executed")
			}
			return nil
		}).Times(3) // A, B, C

		err := s.Run(context.Background(), g, []string{"A"}, 1)
		if err != nil {
			t.Errorf("Run failed: %v", err)
		}
	})
}
