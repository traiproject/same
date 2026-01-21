package scheduler_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"
	"go.trai.ch/same/internal/core/domain"
	"go.uber.org/mock/gomock"
)

// TestScheduler_CacheHydrationFailure verifies that if fetching build info from the store fails,
// the scheduler handles the error gracefully.
func TestScheduler_CacheHydrationFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Scenario: Cache Hydration (Store.Get) fails.
		// Expectation: Run fails gracefully (returns error).

		deps := map[string][]string{"A": {}}
		g := createGraphHelper(t, deps)
		s, m := setupSchedulerTest(t)

		// 1. Env Hydration (Success)
		m.envFactory.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Return([]string{"env=val"}, nil).AnyTimes()

		// 2. Input Resolution (Success)
		m.resolver.EXPECT().ResolveInputs(gomock.Any(), gomock.Any()).Return([]string{}, nil).AnyTimes()
		m.hasher.EXPECT().ComputeInputHash(gomock.Any(), gomock.Any(), gomock.Any()).Return("input_hash", nil).AnyTimes()

		// 3. Store Get Fails
		expectedErr := errors.New("store read failed")
		m.store.EXPECT().Get(gomock.Any()).Return(nil, expectedErr).Times(1)

		// Run
		ctx := context.Background()
		err := s.Run(ctx, g, []string{"all"}, 1, false)

		// Assert
		require.Error(t, err)
		require.True(t,
			errors.Is(err, expectedErr) ||
				errors.Is(err, domain.ErrStoreReadFailed) ||
				errors.Is(err, domain.ErrTaskExecutionFailed),
		)
	})
}

// TestScheduler_EnvironmentPreparationFailure verifies that if environment hydration fails,
// the scheduler fails immediately before execution.
func TestScheduler_EnvironmentPreparationFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Scenario: Environment Preparation failure.
		// Expectation: Run fails immediately.

		// Manual graph setup to include Tools
		g := domain.NewGraph()
		g.SetRoot("/tmp/root")
		tA := &domain.Task{
			Name:    domain.NewInternedString("A"),
			Command: []string{"echo", "A"},
			Tools:   map[string]string{"go": "1.25"},
		}
		require.NoError(t, g.AddTask(tA))
		require.NoError(t, g.Validate())

		s, m := setupSchedulerTest(t)

		// 1. Env Hydration Fails
		expectedErr := errors.New("env factory failed")
		m.envFactory.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Return(nil, expectedErr).Times(1)

		// Run
		ctx := context.Background()
		err := s.Run(ctx, g, []string{"all"}, 1, false)

		// Assert
		require.Error(t, err)
		require.ErrorIs(t, err, expectedErr)
	})
}

// TestScheduler_OutputValidationFailure verifies that the scheduler behaves correctly
// when output validation/hashing fails after execution.
// Note: Currently, the implementation swallows output hash errors (cache miss behavior).
// This test asserts that behavior: success but no cache update.
func TestScheduler_OutputValidationFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Scenario: Output Validation (ComputeOutputHash) fails (e.g. missing output).
		// Expectation: Code currently swallows this (success, no cache update).

		root := t.TempDir()
		outPath := filepath.Join(root, "out")

		// Manual graph setup to include Outputs
		g := domain.NewGraph()
		g.SetRoot(root)
		tA := &domain.Task{
			Name:    domain.NewInternedString("A"),
			Command: []string{"echo", "A"},
			Outputs: []domain.InternedString{domain.NewInternedString(outPath)},
		}
		require.NoError(t, g.AddTask(tA))
		require.NoError(t, g.Validate())

		s, m := setupSchedulerTest(t)

		m.envFactory.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Return([]string{}, nil).AnyTimes()
		m.resolver.EXPECT().ResolveInputs(gomock.Any(), gomock.Any()).Return([]string{}, nil).AnyTimes()
		m.hasher.EXPECT().ComputeInputHash(gomock.Any(), gomock.Any(), gomock.Any()).Return("hash", nil).AnyTimes()
		m.store.EXPECT().Get(gomock.Any()).Return(nil, nil).AnyTimes()

		// Execute succeeds
		m.executor.EXPECT().Execute(
			gomock.Any(),
			matchTask("A"),
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
		).Return(nil).Times(1)

		// Output Hash Fails
		m.hasher.EXPECT().ComputeOutputHash(gomock.Any(), gomock.Any()).Return("", errors.New("output missing")).Times(1)

		// Store.Put should NOT be called because hash computation failed
		m.store.EXPECT().Put(gomock.Any()).Times(0)

		// Run
		ctx := context.Background()
		err := s.Run(ctx, g, []string{"all"}, 1, false)

		// Assert
		require.NoError(t, err)
	})
}

// TestScheduler_ZeroTaskGraph verifies that running with an empty graph or
// no matching targets returns no error (no-op).
func TestScheduler_ZeroTaskGraph(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Scenario: Zero-Task Graph.
		// Expectation: Run returns nil.

		deps := map[string][]string{} // Empty
		g := createGraphHelper(t, deps)
		s, _ := setupSchedulerTest(t)

		ctx := context.Background()
		err := s.Run(ctx, g, []string{"all"}, 1, false)

		require.NoError(t, err)
	})
}
