// Package scheduler implements the task execution scheduler.
package scheduler

import (
	"context"
	"errors"
	"sync"

	"go.trai.ch/bob/internal/core/domain"
	"go.trai.ch/bob/internal/core/ports"
	"go.trai.ch/zerr"
)

// TaskStatus represents the status of a task.
type TaskStatus string

const (
	// StatusPending indicates the task is waiting to be executed.
	StatusPending TaskStatus = "Pending"
	// StatusRunning indicates the task is currently executing.
	StatusRunning TaskStatus = "Running"
	// StatusCompleted indicates the task has finished successfully.
	StatusCompleted TaskStatus = "Completed"
	// StatusFailed indicates the task execution failed.
	StatusFailed TaskStatus = "Failed"
	// StatusCached indicates the task was skipped because it was cached.
	StatusCached TaskStatus = "Cached"
)

// Scheduler manages the execution of tasks in the dependency graph.
type Scheduler struct {
	graph    *domain.Graph
	executor ports.Executor
	hasher   ports.Hasher
	store    ports.BuildInfoStore

	mu         sync.RWMutex
	taskStatus map[domain.InternedString]TaskStatus
}

// NewScheduler creates a new Scheduler with the given graph and executor.
// It validates the graph before proceeding and returns an error if validation fails.
func NewScheduler(
	graph *domain.Graph,
	executor ports.Executor,
	hasher ports.Hasher,
	store ports.BuildInfoStore,
) (*Scheduler, error) {
	// Explicitly validate the graph to ensure executionOrder is populated
	if err := graph.Validate(); err != nil {
		return nil, err
	}

	s := &Scheduler{
		graph:      graph,
		executor:   executor,
		hasher:     hasher,
		store:      store,
		taskStatus: make(map[domain.InternedString]TaskStatus),
	}
	s.initTaskStatuses()
	return s, nil
}

// initTaskStatuses initializes all tasks in the graph to Pending.
func (s *Scheduler) initTaskStatuses() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// We iterate over the graph's tasks to initialize their status.
	// Since Graph doesn't expose tasks directly, we might need to rely on Walk or similar.
	// However, for initialization, we want to set everything to Pending.
	// Let's assume we can iterate via Walk for now, or if Graph exposes keys.
	// Wait, domain.Graph doesn't expose keys directly other than Walk.
	// But Walk requires Validate() to be called first.
	// If Validate hasn't been called, Walk might be empty or undefined.
	// Let's assume the graph is passed in a valid state or we just initialize lazily?
	// The requirement says "Scheduler initializes all tasks to 'Pending'".
	// If I can't iterate keys, I might need to add a method to Graph or just use Walk if it's safe.
	// Looking at graph.go, Walk uses executionOrder which is populated by Validate.
	// So we should probably assume Validate has been called or call it?
	// Or maybe we just initialize the map when we start?
	// For now, let's try to use Walk, assuming the graph is ready.
	// Actually, the user prompt says "Scheduler initializes all tasks to 'Pending'".
	// I'll use Walk for now. If Walk is empty, nothing happens, which is fine for an empty graph.

	for task := range s.graph.Walk() {
		s.taskStatus[task.Name] = StatusPending
	}
}

// updateStatus updates the status of a task.
func (s *Scheduler) updateStatus(name domain.InternedString, status TaskStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.taskStatus[name] = status
}

// Run executes the tasks in the graph with the specified parallelism.
func (s *Scheduler) Run(ctx context.Context, parallelism int) error {
	state := s.newRunState(ctx, parallelism)

	for !state.isDone() {
		state.schedule()

		if state.isDone() {
			break
		}

		if state.ctx.Err() != nil && state.active == 0 {
			return errors.Join(state.errs, state.ctx.Err())
		}

		select {
		case res := <-state.resultsCh:
			state.handleResult(res)
		case <-state.ctx.Done():
		}
	}

	if state.ctx.Err() != nil {
		state.errs = errors.Join(state.errs, state.ctx.Err())
	}

	return state.errs
}

type result struct {
	task domain.InternedString
	err  error
}

type schedulerRunState struct {
	inDegree    map[domain.InternedString]int
	tasks       map[domain.InternedString]domain.Task
	ready       []domain.InternedString
	active      int
	resultsCh   chan result
	errs        error
	ctx         context.Context
	parallelism int
	s           *Scheduler
}

func (s *Scheduler) newRunState(ctx context.Context, parallelism int) *schedulerRunState {
	taskCount := s.graph.TaskCount()
	inDegree := make(map[domain.InternedString]int, taskCount)
	tasks := make(map[domain.InternedString]domain.Task, taskCount)

	for task := range s.graph.Walk() {
		tasks[task.Name] = task
		inDegree[task.Name] = len(task.Dependencies)
	}

	var ready []domain.InternedString
	for name, degree := range inDegree {
		if degree == 0 {
			ready = append(ready, name)
		}
	}

	return &schedulerRunState{
		inDegree:    inDegree,
		tasks:       tasks,
		ready:       ready,
		resultsCh:   make(chan result, parallelism),
		ctx:         ctx,
		parallelism: parallelism,
		s:           s,
	}
}

func (state *schedulerRunState) isDone() bool {
	return state.active == 0 && len(state.ready) == 0
}

func (state *schedulerRunState) schedule() {
	for len(state.ready) > 0 && state.active < state.parallelism && state.ctx.Err() == nil {
		taskName := state.ready[0]
		state.ready = state.ready[1:]

		state.active++
		state.s.updateStatus(taskName, StatusRunning)

		go func(t domain.Task) {
			state.resultsCh <- result{task: t.Name, err: state.s.executor.Execute(state.ctx, &t)}
		}(state.tasks[taskName])
	}
}

func (state *schedulerRunState) handleResult(res result) {
	state.active--
	if res.err != nil {
		wrappedErr := zerr.With(zerr.Wrap(res.err, "task execution failed"), "task", res.task.String())
		state.errs = errors.Join(state.errs, wrappedErr)
		state.s.updateStatus(res.task, StatusFailed)
	} else {
		state.s.updateStatus(res.task, StatusCompleted)
		for _, dep := range state.s.graph.Dependents(res.task) {
			state.inDegree[dep]--
			if state.inDegree[dep] == 0 {
				state.ready = append(state.ready, dep)
			}
		}
	}
}
