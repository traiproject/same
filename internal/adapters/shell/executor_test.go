package shell

import (
	"context"
	"testing"

	"go.trai.ch/bob/internal/core/domain"
	"go.trai.ch/bob/internal/core/ports/mocks"
	"go.uber.org/mock/gomock"
)

func TestExecutor_Execute(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLogger(ctrl)
	executor := NewExecutor(mockLogger)

	t.Run("Success", func(t *testing.T) {
		task := &domain.Task{
			Name:    domain.NewInternedString("test"),
			Command: []string{"echo", "hello"},
		}

		mockLogger.EXPECT().Info("hello")

		if err := executor.Execute(context.Background(), task); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("Failure", func(t *testing.T) {
		task := &domain.Task{
			Name:    domain.NewInternedString("fail"),
			Command: []string{"sh", "-c", "exit 1"},
		}

		if err := executor.Execute(context.Background(), task); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("EmptyCommand", func(t *testing.T) {
		task := &domain.Task{
			Name:    domain.NewInternedString("empty"),
			Command: []string{},
		}

		if err := executor.Execute(context.Background(), task); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
