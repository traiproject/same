package scheduler_test

import (
	"context"
	"errors"
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
