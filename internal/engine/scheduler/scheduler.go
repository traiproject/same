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
)

// Scheduler manages the execution of tasks in the dependency graph.
type Scheduler struct {
	executor ports.Executor

	mu         sync.RWMutex
	taskStatus map[domain.InternedString]TaskStatus
}

// NewScheduler creates a new Scheduler with the given executor.
func NewScheduler(executor ports.Executor) *Scheduler {
	s := &Scheduler{
		executor:   executor,
		taskStatus: make(map[domain.InternedString]TaskStatus),
	}
	return s
}

// initTaskStatuses initializes the status of tasks in the graph to Pending.
func (s *Scheduler) initTaskStatuses(tasks []domain.InternedString) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, task := range tasks {
		s.taskStatus[task] = StatusPending
	}
}

// updateStatus updates the status of a task.
func (s *Scheduler) updateStatus(name domain.InternedString, status TaskStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.taskStatus[name] = status
}

// Run executes the tasks in the graph with the specified parallelism.
// If targetNames is empty or contains "all", all tasks in the graph are executed.
// Otherwise, only the specified tasks and their dependencies are executed.
func (s *Scheduler) Run(ctx context.Context, graph *domain.Graph, targetNames []string, parallelism int) error {
	// Explicitly validate the graph to ensure executionOrder is populated
	if err := graph.Validate(); err != nil {
		return err
	}

	state, err := s.newRunState(ctx, graph, targetNames, parallelism)
	if err != nil {
		return err
	}

	s.initTaskStatuses(state.allTasks)

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
	graph       *domain.Graph
	inDegree    map[domain.InternedString]int
	tasks       map[domain.InternedString]domain.Task
	ready       []domain.InternedString
	active      int
	resultsCh   chan result
	errs        error
	ctx         context.Context
	parallelism int
	s           *Scheduler
	allTasks    []domain.InternedString
}

func (s *Scheduler) newRunState(
	ctx context.Context,
	graph *domain.Graph,
	targetNames []string,
	parallelism int,
) (*schedulerRunState, error) {
	tasksToRun, allTasks, err := s.resolveTasksToRun(graph, targetNames)
	if err != nil {
		return nil, err
	}

	taskCount := len(tasksToRun)
	inDegree := make(map[domain.InternedString]int, taskCount)
	tasks := make(map[domain.InternedString]domain.Task, taskCount)

	for name := range tasksToRun {
		task, _ := graph.GetTask(name)
		tasks[name] = task

		// Calculate in-degree based only on dependencies that are also in tasksToRun
		degree := 0
		for _, dep := range task.Dependencies {
			if tasksToRun[dep] {
				degree++
			}
		}
		inDegree[name] = degree
	}

	var ready []domain.InternedString
	for name, degree := range inDegree {
		if degree == 0 {
			ready = append(ready, name)
		}
	}

	return &schedulerRunState{
		graph:       graph,
		inDegree:    inDegree,
		tasks:       tasks,
		ready:       ready,
		resultsCh:   make(chan result, parallelism),
		ctx:         ctx,
		parallelism: parallelism,
		s:           s,
		allTasks:    allTasks,
	}, nil
}

func (s *Scheduler) resolveTasksToRun(
	graph *domain.Graph,
	targetNames []string,
) (map[domain.InternedString]bool, []domain.InternedString, error) {
	runAll := len(targetNames) == 0
	if !runAll {
		for _, name := range targetNames {
			if name == "all" {
				runAll = true
				break
			}
		}
	}

	if runAll {
		return s.resolveAllTasks(graph)
	}
	return s.resolveTargetTasks(graph, targetNames)
}

func (s *Scheduler) resolveAllTasks(
	graph *domain.Graph,
) (map[domain.InternedString]bool, []domain.InternedString, error) {
	tasksToRun := make(map[domain.InternedString]bool)
	allTasks := make([]domain.InternedString, 0, graph.TaskCount())
	for task := range graph.Walk() {
		tasksToRun[task.Name] = true
		allTasks = append(allTasks, task.Name)
	}
	return tasksToRun, allTasks, nil
}

func (s *Scheduler) resolveTargetTasks(
	graph *domain.Graph,
	targetNames []string,
) (map[domain.InternedString]bool, []domain.InternedString, error) {
	tasksToRun := make(map[domain.InternedString]bool)
	var allTasks []domain.InternedString

	for _, nameStr := range targetNames {
		name := domain.NewInternedString(nameStr)
		if _, ok := graph.GetTask(name); !ok {
			return nil, nil, zerr.With(domain.ErrTaskNotFound, "task", name.String())
		}

		if !tasksToRun[name] {
			tasksToRun[name] = true
			allTasks = append(allTasks, name)
		}
	}

	return tasksToRun, allTasks, nil
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
		for _, dep := range state.graph.Dependents(res.task) {
			// Only consider dependents that are part of the current execution
			if _, ok := state.tasks[dep]; ok {
				state.inDegree[dep]--
				if state.inDegree[dep] == 0 {
					state.ready = append(state.ready, dep)
				}
			}
		}
	}
}
