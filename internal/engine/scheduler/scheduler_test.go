package scheduler

import (
	"context"
	"testing"
	"testing/synctest"

	"go.trai.ch/bob/internal/core/domain"
)

type MockExecutor struct {
	ExecuteCalled chan *domain.Task
	FinishExecute chan error
}

func NewMockExecutor() *MockExecutor {
	return &MockExecutor{
		ExecuteCalled: make(chan *domain.Task, 1),
		FinishExecute: make(chan error, 1),
	}
}

func (m *MockExecutor) Execute(ctx context.Context, task *domain.Task) error {
	select {
	case m.ExecuteCalled <- task:
	case <-ctx.Done():
		return ctx.Err()
	}

	select {
	case err := <-m.FinishExecute:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestScheduler_Init(t *testing.T) {
	synctest.Run(func() {
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

		if err := g.Validate(); err != nil {
			t.Fatalf("failed to validate graph: %v", err)
		}

		mockExec := NewMockExecutor()
		s := NewScheduler(g, mockExec)

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
