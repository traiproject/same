package scheduler_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"
	"go.trai.ch/same/internal/core/domain"
	"go.trai.ch/same/internal/core/ports"
	"go.trai.ch/same/internal/core/ports/mocks"
	"go.trai.ch/same/internal/engine/scheduler"
	"go.uber.org/mock/gomock"
)

type schedulerTestMocks struct {
	executor   *mocks.MockExecutor
	store      *mocks.MockBuildInfoStore
	hasher     *mocks.MockHasher
	resolver   *mocks.MockInputResolver
	tracer     *mocks.MockTracer
	envFactory *mocks.MockEnvironmentFactory
}

// setupSchedulerTest creates a scheduler and common mocks.
func setupSchedulerTest(t *testing.T) (*scheduler.Scheduler, schedulerTestMocks) {
	t.Helper()
	ctrl := gomock.NewController(t)
	m := schedulerTestMocks{
		executor:   mocks.NewMockExecutor(ctrl),
		store:      mocks.NewMockBuildInfoStore(ctrl),
		hasher:     mocks.NewMockHasher(ctrl),
		resolver:   mocks.NewMockInputResolver(ctrl),
		tracer:     mocks.NewMockTracer(ctrl),
		envFactory: mocks.NewMockEnvironmentFactory(ctrl),
	}

	// Default optimistic mocks to reduce noise in specific tests.
	mockSpan := mocks.NewMockSpan(ctrl)
	mockSpan.EXPECT().End().AnyTimes()
	mockSpan.EXPECT().RecordError(gomock.Any()).AnyTimes()
	mockSpan.EXPECT().SetAttribute(gomock.Any(), gomock.Any()).AnyTimes()

	// Start has variadic signature: Start(ctx, name, ...opts).
	m.tracer.EXPECT().Start(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, _ string, _ ...ports.SpanOption) (context.Context, ports.Span) {
			return ctx, mockSpan
		},
	).AnyTimes()
	m.tracer.EXPECT().EmitPlan(gomock.Any(), gomock.Any()).AnyTimes()

	s := scheduler.NewScheduler(m.executor, m.store, m.hasher, m.resolver, m.tracer, m.envFactory)
	return s, m
}

// createGraphHelper constructs a graph from a simple map of dependencies.
// deps format: "target" -> ["dep1", "dep2"].
func createGraphHelper(t *testing.T, deps map[string][]string) *domain.Graph {
	t.Helper()
	g := domain.NewGraph()
	g.SetRoot("/tmp/root")

	for name, myDeps := range deps {
		dependents := make([]domain.InternedString, len(myDeps))
		for i, d := range myDeps {
			dependents[i] = domain.NewInternedString(d)
		}

		task := &domain.Task{
			Name:         domain.NewInternedString(name),
			Dependencies: dependents,
			Command:      []string{"echo", name},
		}
		err := g.AddTask(task)
		require.NoError(t, err)
	}

	// Add any dependencies that weren't explicitly keys in the map
	for _, myDeps := range deps {
		for _, d := range myDeps {
			if _, ok := g.GetTask(domain.NewInternedString(d)); !ok {
				task := &domain.Task{
					Name:    domain.NewInternedString(d),
					Command: []string{"echo", d},
				}
				err := g.AddTask(task)
				require.NoError(t, err)
			}
		}
	}

	err := g.Validate()
	require.NoError(t, err)
	return g
}

// taskMatcher implements gomock.Matcher for domain.Task.
type taskMatcher struct {
	name string
}

func (m taskMatcher) Matches(x interface{}) bool {
	t, ok := x.(*domain.Task)
	if !ok {
		return false
	}
	return t.Name.String() == m.name
}

func (m taskMatcher) String() string {
	return "task name is " + m.name
}

func matchTask(name string) gomock.Matcher {
	return taskMatcher{name: name}
}

func TestScheduler_DiamondDependency(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Graph: A -> B, A -> C, B -> D, C -> D
		// Execution Order should be: D -> (B, C parallel) -> A.
		deps := map[string][]string{
			"A": {"B", "C"},
			"B": {"D"},
			"C": {"D"},
		}
		g := createGraphHelper(t, deps)
		s, m := setupSchedulerTest(t)

		// Mocks setup
		// 1. Env Hydration.
		m.envFactory.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Return([]string{}, nil).AnyTimes()

		// 2. Input Hashing (Check Cache) - Assume all miss cache for this test.
		m.resolver.EXPECT().ResolveInputs(gomock.Any(), gomock.Any()).Return([]string{}, nil).AnyTimes()
		m.hasher.EXPECT().ComputeInputHash(gomock.Any(), gomock.Any(), gomock.Any()).Return("hash", nil).AnyTimes()
		m.store.EXPECT().Get(gomock.Any()).Return(nil, nil).AnyTimes()
		m.hasher.EXPECT().ComputeOutputHash(gomock.Any(), gomock.Any()).Return("out_hash", nil).AnyTimes()
		m.store.EXPECT().Put(gomock.Any()).Return(nil).AnyTimes()

		// 3. Execution Assertions.
		// Check that D runs exactly once.
		dCall := m.executor.EXPECT().Execute(
			gomock.Any(),
			matchTask("D"),
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
		).Return(nil).Times(1)

		// Check B and C run after D.
		bCall := m.executor.EXPECT().Execute(
			gomock.Any(),
			matchTask("B"),
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
		).Return(nil).Times(1).After(dCall)

		cCall := m.executor.EXPECT().Execute(
			gomock.Any(),
			matchTask("C"),
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
		).Return(nil).Times(1).After(dCall)

		// Check A runs after B and C.
		m.executor.EXPECT().Execute(
			gomock.Any(),
			matchTask("A"),
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
		).Return(nil).Times(1).After(bCall).After(cCall)

		ctx := context.Background()
		err := s.Run(ctx, g, []string{"all"}, 4, false)
		require.NoError(t, err)
	})
}

func TestScheduler_FailurePropagation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Graph: A -> B. B fails. A should not run.
		deps := map[string][]string{
			"A": {"B"},
		}
		g := createGraphHelper(t, deps)
		s, m := setupSchedulerTest(t)

		// Mocks setup.
		m.envFactory.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Return([]string{}, nil).AnyTimes()
		m.resolver.EXPECT().ResolveInputs(gomock.Any(), gomock.Any()).Return([]string{}, nil).AnyTimes()
		m.hasher.EXPECT().ComputeInputHash(gomock.Any(), gomock.Any(), gomock.Any()).Return("hash", nil).AnyTimes()
		m.store.EXPECT().Get(gomock.Any()).Return(nil, nil).AnyTimes()

		// B fails.
		failureErr := errors.New("boom")
		m.executor.EXPECT().Execute(
			gomock.Any(),
			matchTask("B"),
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
		).Return(failureErr).Times(1)

		// A should NOT run.
		m.executor.EXPECT().Execute(
			gomock.Any(),
			matchTask("A"),
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
		).Times(0)

		ctx := context.Background()
		err := s.Run(ctx, g, []string{"all"}, 4, false)
		require.Error(t, err)
		require.True(t, errors.Is(err, failureErr) || errors.Is(err, domain.ErrTaskExecutionFailed))
	})
}

func TestScheduler_Cancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Graph: A (long running). Cancel context.
		deps := map[string][]string{
			"A": {},
		}
		g := createGraphHelper(t, deps)
		s, m := setupSchedulerTest(t)

		m.envFactory.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Return([]string{}, nil).AnyTimes()
		m.resolver.EXPECT().ResolveInputs(gomock.Any(), gomock.Any()).Return([]string{}, nil).AnyTimes()
		m.hasher.EXPECT().ComputeInputHash(gomock.Any(), gomock.Any(), gomock.Any()).Return("hash", nil).AnyTimes()
		m.store.EXPECT().Get(gomock.Any()).Return(nil, nil).AnyTimes()

		// Wait, executor context should be canceled.
		m.executor.EXPECT().Execute(
			gomock.Any(),
			matchTask("A"),
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
		).DoAndReturn(
			func(ctx context.Context, _ *domain.Task, _ []string, _, _ interface{}) error {
				<-ctx.Done()
				return ctx.Err()
			},
		).Times(1)

		ctx, cancel := context.WithCancel(context.Background())

		// Run in a goroutine so we can cancel it.
		errCh := make(chan error, 1) // Buffered.
		go func() {
			errCh <- s.Run(ctx, g, []string{"all"}, 4, false)
		}()

		// Give it a moment to start.
		synctest.Wait()

		cancel()
		synctest.Wait()

		err := <-errCh
		require.Error(t, err)
		require.ErrorIs(t, err, context.Canceled)
	})
}

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

func TestScheduler_Coverage(t *testing.T) {
	t.Run("CacheVerification", func(t *testing.T) {
		t.Run("PartialCacheMiss_OutputHashMismatch", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				// Scenario: Input hash matches, but outputs don't match stored info
				root := t.TempDir()
				g := domain.NewGraph()
				g.SetRoot(root)
				output := filepath.Join(root, "out")
				task := &domain.Task{
					Name:    domain.NewInternedString("A"),
					Command: []string{"echo", "A"},
					Outputs: []domain.InternedString{domain.NewInternedString(output)},
				}
				require.NoError(t, g.AddTask(task))
				require.NoError(t, g.Validate())

				s, m := setupSchedulerTest(t)

				// Environment & Input Resolution
				m.envFactory.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Return([]string{}, nil).AnyTimes()
				m.resolver.EXPECT().ResolveInputs(gomock.Any(), gomock.Any()).Return([]string{}, nil).AnyTimes()
				m.hasher.EXPECT().ComputeInputHash(
					gomock.Any(), gomock.Any(), gomock.Any(),
				).Return("input_hash_123", nil).AnyTimes()

				// Store returns a hit
				m.store.EXPECT().Get("A").Return(&domain.BuildInfo{
					TaskName:   "A",
					InputHash:  "input_hash_123",
					OutputHash: "expected_output_hash",
				}, nil).Times(1)

				// Recalculating output hash returns different value (mismatch)
				m.hasher.EXPECT().ComputeOutputHash(gomock.Any(), gomock.Any()).Return("actual_output_hash_mismatch", nil).Times(1)

				// Should trigger execution because cache check failed
				m.executor.EXPECT().Execute(
					gomock.Any(), matchTask("A"), gomock.Any(), gomock.Any(), gomock.Any(),
				).Return(nil).Times(1)

				// Should update store with new result
				m.store.EXPECT().Put(gomock.Any()).Return(nil).Times(1)
				// Re-calculating output hash for the Put
				m.hasher.EXPECT().ComputeOutputHash(
					gomock.Any(), gomock.Any(),
				).Return("new_output_hash", nil).Times(1)

				err := s.Run(context.Background(), g, []string{"all"}, 1, false)
				require.NoError(t, err)
			})
		})

		t.Run("MissingOutputArtifacts", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				// Scenario: Input hash matches, but checking outputs returns error (files missing)
				root := t.TempDir()
				g := domain.NewGraph()
				g.SetRoot(root)
				output := filepath.Join(root, "out")
				task := &domain.Task{
					Name:    domain.NewInternedString("A"),
					Command: []string{"echo", "A"},
					Outputs: []domain.InternedString{domain.NewInternedString(output)},
				}
				require.NoError(t, g.AddTask(task))
				require.NoError(t, g.Validate())

				s, m := setupSchedulerTest(t)

				m.envFactory.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Return([]string{}, nil).AnyTimes()
				m.resolver.EXPECT().ResolveInputs(gomock.Any(), gomock.Any()).Return([]string{}, nil).AnyTimes()
				m.hasher.EXPECT().ComputeInputHash(
					gomock.Any(), gomock.Any(), gomock.Any(),
				).Return("input_hash_123", nil).AnyTimes()

				m.store.EXPECT().Get("A").Return(&domain.BuildInfo{
					TaskName:   "A",
					InputHash:  "input_hash_123",
					OutputHash: "expected_output_hash",
				}, nil).Times(1)

				// ComputeOutputHash fails (e.g., file missing)
				m.hasher.EXPECT().ComputeOutputHash(gomock.Any(), gomock.Any()).Return("", errors.New("file not found")).Times(1)

				// Expect execution
				m.executor.EXPECT().Execute(
					gomock.Any(), matchTask("A"), gomock.Any(), gomock.Any(), gomock.Any(),
				).Return(nil).Times(1)
				m.store.EXPECT().Put(gomock.Any()).Return(nil).AnyTimes()
				m.hasher.EXPECT().ComputeOutputHash(gomock.Any(), gomock.Any()).Return("new_hash", nil).AnyTimes()

				err := s.Run(context.Background(), g, []string{"all"}, 1, false)
				require.NoError(t, err)
			})
		})

		t.Run("StoreReadError", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				deps := map[string][]string{"A": {}}
				g := createGraphHelper(t, deps)
				s, m := setupSchedulerTest(t)

				m.envFactory.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Return([]string{}, nil).AnyTimes()
				m.resolver.EXPECT().ResolveInputs(gomock.Any(), gomock.Any()).Return([]string{}, nil).AnyTimes()
				m.hasher.EXPECT().ComputeInputHash(gomock.Any(), gomock.Any(), gomock.Any()).Return("hash", nil).AnyTimes()

				// Store.Get fails
				storeErr := errors.New("db connection failed")
				m.store.EXPECT().Get("A").Return(nil, storeErr).Times(1)

				// Should NOT execute, but fail immediately
				m.executor.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

				err := s.Run(context.Background(), g, []string{"all"}, 1, false)
				require.Error(t, err)
				require.True(t, errors.Is(err, storeErr) || errors.Is(err, domain.ErrStoreReadFailed))
			})
		})
	})

	t.Run("OutputValidation", func(t *testing.T) {
		t.Run("PathTraversal", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				// Construct a graph with a task having invalid output path
				root := t.TempDir()
				g := domain.NewGraph()
				g.SetRoot(root)

				// Resolve "../secret.txt" relative to root
				// On simple FS, this works.
				badOutput := filepath.Join(root, "..", "secret.txt")

				task := &domain.Task{
					Name:    domain.NewInternedString("bad_task"),
					Command: []string{"echo"},
					// Output outside root
					Outputs: []domain.InternedString{domain.NewInternedString(badOutput)},
				}
				require.NoError(t, g.AddTask(task))
				require.NoError(t, g.Validate())

				s, m := setupSchedulerTest(t)

				m.envFactory.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Return([]string{}, nil).AnyTimes()
				m.resolver.EXPECT().ResolveInputs(gomock.Any(), gomock.Any()).Return([]string{}, nil).AnyTimes()
				m.hasher.EXPECT().ComputeInputHash(gomock.Any(), gomock.Any(), gomock.Any()).Return("hash", nil).AnyTimes()
				m.store.EXPECT().Get(gomock.Any()).Return(nil, nil).AnyTimes()

				// Execution fails at validation step, so Executor.Execute is NOT called
				m.executor.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

				err := s.Run(context.Background(), g, []string{"all"}, 1, false)
				require.Error(t, err)
				require.ErrorContains(t, err, domain.ErrOutputPathOutsideRoot.Error())
			})
		})

		t.Run("CleanFailure", func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				// To simulate clean failure (os.RemoveAll), we need a task with an output
				// in a directory heavily restricted or problematic.
				// Since we can't easily mock `os`, we create a temp dir with read-only permissions.

				tmpRoot := t.TempDir()

				// Create a subdirectory that we will make immutable
				lockedDir := filepath.Join(tmpRoot, "locked")
				require.NoError(t, os.Mkdir(lockedDir, 0o700))

				// We want the task to output to 'locked/output.txt'
				// But first, we need to ensure 'locked' prevents deletion of 'output.txt' IF it exists?
				// Actually, os.RemoveAll on 'locked/output.txt' requires Write permission on 'locked'.
				// So we create 'locked/output.txt' and then chmod 'locked' to 0500 (Read+Exec, No Write).

				outputFile := filepath.Join(lockedDir, "output.txt")
				require.NoError(t, os.WriteFile(outputFile, []byte("data"), 0o600))

				// Remove write permission from parent directory
				//nolint:gosec // Read-only directory for test; execute needed for traversal
				require.NoError(t, os.Chmod(lockedDir, 0o500))
				t.Cleanup(func() {
					//nolint:gosec // Restore permissions
					_ = os.Chmod(lockedDir, 0o755)
				})

				g := domain.NewGraph()
				g.SetRoot(tmpRoot)
				task := &domain.Task{
					Name:    domain.NewInternedString("clean_fail_task"),
					Command: []string{"echo"},
					Outputs: []domain.InternedString{domain.NewInternedString(outputFile)},
				}
				require.NoError(t, g.AddTask(task))
				require.NoError(t, g.Validate())

				s, m := setupSchedulerTest(t)
				m.envFactory.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Return([]string{}, nil).AnyTimes()
				m.resolver.EXPECT().ResolveInputs(gomock.Any(), gomock.Any()).Return([]string{}, nil).AnyTimes()
				m.hasher.EXPECT().ComputeInputHash(gomock.Any(), gomock.Any(), gomock.Any()).Return("hash", nil).AnyTimes()
				m.store.EXPECT().Get(gomock.Any()).Return(nil, nil).AnyTimes()

				// Executor should NOT be called because Clean fails
				m.executor.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

				err := s.Run(context.Background(), g, []string{"all"}, 1, false)
				require.Error(t, err)
				require.Contains(t, err.Error(), domain.ErrFailedToCleanOutput.Error())
			})
		})
	})

	t.Run("ForcedExecution", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			root := t.TempDir()
			g := domain.NewGraph()
			g.SetRoot(root)
			output := filepath.Join(root, "out")
			task := &domain.Task{
				Name:    domain.NewInternedString("A"),
				Command: []string{"echo", "A"},
				Outputs: []domain.InternedString{domain.NewInternedString(output)},
			}
			require.NoError(t, g.AddTask(task))
			require.NoError(t, g.Validate())

			s, m := setupSchedulerTest(t)

			m.envFactory.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Return([]string{}, nil).AnyTimes()
			m.resolver.EXPECT().ResolveInputs(gomock.Any(), gomock.Any()).Return([]string{}, nil).AnyTimes()

			// Crucial: ComputeInputHash is called, but Store.Get is NEVER called
			m.hasher.EXPECT().ComputeInputHash(gomock.Any(), gomock.Any(), gomock.Any()).Return("hash", nil).Times(1)
			m.store.EXPECT().Get(gomock.Any()).Times(0)

			// Executor IS called
			m.executor.EXPECT().Execute(
				gomock.Any(), matchTask("A"), gomock.Any(), gomock.Any(), gomock.Any(),
			).Return(nil).Times(1)

			// Store.Put is still called to cache the result of the forced run
			m.store.EXPECT().Put(gomock.Any()).Return(nil).Times(1)
			m.hasher.EXPECT().ComputeOutputHash(gomock.Any(), gomock.Any()).Return("out", nil).Times(1)

			// Run with noCache = true
			err := s.Run(context.Background(), g, []string{"all"}, 1, true)
			require.NoError(t, err)
		})
	})

	t.Run("EnvironmentHydrationError", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			g := domain.NewGraph()
			g.SetRoot("/tmp/root")
			task := &domain.Task{
				Name:    domain.NewInternedString("tool_task"),
				Command: []string{"echo"},
				Tools:   map[string]string{"go": "1.21"},
			}
			require.NoError(t, g.AddTask(task))
			require.NoError(t, g.Validate())

			s, m := setupSchedulerTest(t)

			envErr := errors.New("nix error")
			// Hydration fails
			m.envFactory.EXPECT().GetEnvironment(gomock.Any(), gomock.Eq(task.Tools)).Return(nil, envErr).Times(1)

			err := s.Run(context.Background(), g, []string{"all"}, 1, false)
			require.Error(t, err)
			require.ErrorIs(t, err, envErr) // or wrapped
		})
	})
}
