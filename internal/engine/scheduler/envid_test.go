package scheduler_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/bob/internal/core/domain"
	"go.trai.ch/bob/internal/core/ports/mocks"
	"go.trai.ch/bob/internal/engine/scheduler"
	"go.uber.org/mock/gomock"
)

func TestScheduler_EnvironmentIDGeneration(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExec := mocks.NewMockExecutor(ctrl)
	mockStore := mocks.NewMockBuildInfoStore(ctrl)
	mockHasher := mocks.NewMockHasher(ctrl)
	mockResolver := mocks.NewMockInputResolver(ctrl)
	mockTracer := mocks.NewMockTracer(ctrl)

	s := scheduler.NewScheduler(mockExec, mockStore, mockHasher, mockResolver, mockTracer, nil)

	t.Run("IdenticalToolsSameEnvID", func(t *testing.T) {
		// Two tasks with identical tools should get the same EnvID
		g := domain.NewGraph()
		g.SetRoot(".")

		tools := map[string]string{
			"go":            "go@1.25.4",
			"golangci-lint": "golangci-lint@2.6.2",
		}

		task1 := &domain.Task{
			Name:  domain.NewInternedString("task1"),
			Tools: tools,
		}
		task2 := &domain.Task{
			Name:  domain.NewInternedString("task2"),
			Tools: tools,
		}
		_ = g.AddTask(task1)
		_ = g.Validate()
		_ = g.AddTask(task2)
		_ = g.Validate()

		ctx := context.Background()
		taskEnvIDs, err := s.GetTaskEnvIDs(ctx, g, []string{"all"}, 1, false)
		require.NoError(t, err)

		require.Contains(t, taskEnvIDs, task1.Name)
		require.Contains(t, taskEnvIDs, task2.Name)

		envID1 := taskEnvIDs[task1.Name]
		envID2 := taskEnvIDs[task2.Name]

		assert.Equal(t, envID1, envID2, "Tasks with identical tools should have the same EnvID")
		assert.NotEmpty(t, envID1, "EnvID should not be empty")
	})

	t.Run("DifferentEnvIDs", func(t *testing.T) {
		tests := []struct {
			name   string
			tools1 map[string]string
			tools2 map[string]string
			reason string
		}{
			{
				name: "DifferentTools",
				tools1: map[string]string{
					"go": "go@1.25.4",
				},
				tools2: map[string]string{
					"python": "python@3.12",
				},
				reason: "Tasks with different tools should have different EnvIDs",
			},
			{
				name: "DifferentVersions",
				tools1: map[string]string{
					"go": "go@1.25.4",
				},
				tools2: map[string]string{
					"go": "go@1.24.0",
				},
				reason: "Same tool with different versions should have different EnvIDs",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				g := domain.NewGraph()
				g.SetRoot(".")

				task1 := &domain.Task{
					Name:  domain.NewInternedString("task1"),
					Tools: tt.tools1,
				}
				task2 := &domain.Task{
					Name:  domain.NewInternedString("task2"),
					Tools: tt.tools2,
				}
				_ = g.AddTask(task1)
				_ = g.Validate()
				_ = g.AddTask(task2)
				_ = g.Validate()

				ctx := context.Background()
				taskEnvIDs, err := s.GetTaskEnvIDs(ctx, g, []string{"all"}, 1, false)
				require.NoError(t, err)

				require.Contains(t, taskEnvIDs, task1.Name)
				require.Contains(t, taskEnvIDs, task2.Name)

				envID1 := taskEnvIDs[task1.Name]
				envID2 := taskEnvIDs[task2.Name]

				assert.NotEqual(t, envID1, envID2, tt.reason)
			})
		}
	})

	t.Run("NoToolsNoEnvID", func(t *testing.T) {
		// Tasks without tools should not have EnvIDs
		g := domain.NewGraph()
		g.SetRoot(".")

		task1 := &domain.Task{
			Name:  domain.NewInternedString("task1"),
			Tools: nil,
		}
		task2 := &domain.Task{
			Name:  domain.NewInternedString("task2"),
			Tools: map[string]string{},
		}
		_ = g.AddTask(task1)
		_ = g.Validate()
		_ = g.AddTask(task2)
		_ = g.Validate()

		ctx := context.Background()
		taskEnvIDs, err := s.GetTaskEnvIDs(ctx, g, []string{"all"}, 1, false)
		require.NoError(t, err)

		assert.NotContains(t, taskEnvIDs, task1.Name, "Task without tools should not have EnvID")
		assert.NotContains(t, taskEnvIDs, task2.Name, "Task with empty tools should not have EnvID")
	})

	t.Run("OrderIndependentEnvID", func(t *testing.T) {
		// Tools map with different key insertion order should produce the same EnvID
		g := domain.NewGraph()
		g.SetRoot(".")

		// Create two maps with same content but potentially different iteration order
		tools1 := map[string]string{
			"go":            "go@1.25.4",
			"golangci-lint": "golangci-lint@2.6.2",
			"python":        "python@3.12",
		}
		tools2 := map[string]string{
			"python":        "python@3.12",
			"go":            "go@1.25.4",
			"golangci-lint": "golangci-lint@2.6.2",
		}

		task1 := &domain.Task{
			Name:  domain.NewInternedString("task1"),
			Tools: tools1,
		}
		task2 := &domain.Task{
			Name:  domain.NewInternedString("task2"),
			Tools: tools2,
		}
		_ = g.AddTask(task1)
		_ = g.Validate()
		_ = g.AddTask(task2)
		_ = g.Validate()

		ctx := context.Background()
		taskEnvIDs, err := s.GetTaskEnvIDs(ctx, g, []string{"all"}, 1, false)
		require.NoError(t, err)

		require.Contains(t, taskEnvIDs, task1.Name)
		require.Contains(t, taskEnvIDs, task2.Name)

		envID1 := taskEnvIDs[task1.Name]
		envID2 := taskEnvIDs[task2.Name]

		assert.Equal(t, envID1, envID2, "EnvID should be deterministic regardless of map iteration order")
	})
}
