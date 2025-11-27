// Package scheduler implements the task execution scheduler.
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

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
	store    ports.BuildInfoStore
	hasher   ports.Hasher
	resolver ports.InputResolver
	logger   ports.Logger

	mu         sync.RWMutex
	taskStatus map[domain.InternedString]TaskStatus
}

// NewScheduler creates a new Scheduler with the given executor, store, hasher, and logger.
func NewScheduler(
	executor ports.Executor,
	store ports.BuildInfoStore,
	hasher ports.Hasher,
	resolver ports.InputResolver,
	logger ports.Logger,
) *Scheduler {
	s := &Scheduler{
		executor:   executor,
		store:      store,
		hasher:     hasher,
		resolver:   resolver,
		logger:     logger,
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
// If targetNames contains "all", all tasks in the graph are executed.
// Otherwise, only the specified tasks are executed.
// If force is true, cache is bypassed and all tasks are executed.
func (s *Scheduler) Run(
	ctx context.Context,
	graph *domain.Graph,
	targetNames []string,
	parallelism int,
	force bool,
) error {
	// Explicitly validate the graph to ensure executionOrder is populated
	if err := graph.Validate(); err != nil {
		return err
	}

	state, err := s.newRunState(ctx, graph, targetNames, parallelism, force)
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
	task        domain.InternedString
	err         error
	skipped     bool
	inputHash   string
	taskOutputs []string
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
	force       bool
}

func (s *Scheduler) newRunState(
	ctx context.Context,
	graph *domain.Graph,
	targetNames []string,
	parallelism int,
	force bool,
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
		force:       force,
	}, nil
}

func (s *Scheduler) resolveTasksToRun(
	graph *domain.Graph,
	targetNames []string,
) (map[domain.InternedString]bool, []domain.InternedString, error) {
	runAll := slices.Contains(targetNames, "all")

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
	targets := make([]domain.InternedString, 0, len(targetNames))
	for _, nameStr := range targetNames {
		name := domain.NewInternedString(nameStr)
		if _, ok := graph.GetTask(name); !ok {
			return nil, nil, zerr.With(domain.ErrTaskNotFound, "task", name.String())
		}
		targets = append(targets, name)
	}

	return s.collectDependencies(graph, targets)
}

func (s *Scheduler) collectDependencies(
	graph *domain.Graph,
	targets []domain.InternedString,
) (map[domain.InternedString]bool, []domain.InternedString, error) {
	tasksToRun := make(map[domain.InternedString]bool)
	var allTasks []domain.InternedString

	// Use a queue for BFS to collect all dependencies
	queue := make([]domain.InternedString, len(targets))
	copy(queue, targets)

	visited := make(map[domain.InternedString]bool)
	for _, t := range targets {
		visited[t] = true
	}

	for len(queue) > 0 {
		currentName := queue[0]
		queue = queue[1:]

		// Add to tasks to run
		if !tasksToRun[currentName] {
			tasksToRun[currentName] = true
			allTasks = append(allTasks, currentName)
		}

		task, _ := graph.GetTask(currentName)
		for _, dep := range task.Dependencies {
			if !visited[dep] {
				visited[dep] = true
				queue = append(queue, dep)
			}
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
			var hash string
			var err error

			if state.force {
				hash, err = state.s.computeHashForce(&t, state.graph.Root())
				if err != nil {
					state.resultsCh <- result{task: t.Name, err: err}
					return
				}
				// Bypass cache, always execute
			} else {
				// Normal mode: check cache
				skipped, h, err := state.s.checkTaskCache(state.ctx, &t, state.graph.Root())
				hash = h
				if err != nil {
					state.resultsCh <- result{task: t.Name, err: err}
					return
				}

				if skipped {
					state.resultsCh <- result{task: t.Name, skipped: true, inputHash: hash}
					return
				}
			}

			// Convert outputs to string slice for result
			outputs := make([]string, len(t.Outputs))
			for i, out := range t.Outputs {
				outputs[i] = out.String()
			}

			state.resultsCh <- result{
				task:        t.Name,
				err:         state.s.executor.Execute(state.ctx, &t),
				inputHash:   hash,
				taskOutputs: outputs,
			}
		}(state.tasks[taskName])
	}
}

func (state *schedulerRunState) handleResult(res result) {
	state.active--

	if res.err != nil {
		// Enhance error with task name
		enhancedErr := zerr.With(zerr.Wrap(res.err, "task execution failed"), "task", res.task.String())
		state.errs = errors.Join(state.errs, enhancedErr)
		state.s.updateStatus(res.task, StatusFailed)
	} else {
		state.handleSuccess(res)
	}
}

func (state *schedulerRunState) handleSuccess(res result) {
	state.s.updateStatus(res.task, StatusCompleted)
	if res.skipped {
		state.s.logger.Info(fmt.Sprintf("Skipping %s (cached)", res.task))
	} else {
		outputHash := state.computeOutputHash(res)
		if outputHash != "" || len(res.taskOutputs) == 0 {
			err := state.s.store.Put(domain.BuildInfo{
				TaskName:   res.task.String(),
				InputHash:  res.inputHash,
				OutputHash: outputHash,
				Timestamp:  time.Now(),
			})
			if err != nil {
				// We log the error but don't fail the build if cache update fails
				state.s.logger.Error(zerr.With(zerr.Wrap(err, "failed to update build info store"), "task", res.task.String()))
			}
		}
	}

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

func (state *schedulerRunState) computeOutputHash(res result) string {
	if len(res.taskOutputs) == 0 {
		return ""
	}

	outputHash, err := state.s.hasher.ComputeOutputHash(res.taskOutputs, state.graph.Root())
	if err != nil {
		state.s.logger.Error(zerr.With(zerr.Wrap(err, "failed to compute output hash"), "task", res.task.String()))
		return ""
	}
	return outputHash
}

// computeHashForce computes input hash in force mode (bypassing cache).
func (s *Scheduler) computeHashForce(task *domain.Task, root string) (string, error) {
	// Step A: Resolve Inputs
	inputs := make([]string, len(task.Inputs))
	for i, input := range task.Inputs {
		inputs[i] = input.String()
	}
	resolvedInputs, err := s.resolver.ResolveInputs(inputs, root)
	if err != nil {
		return "", zerr.Wrap(err, "failed to resolve inputs")
	}

	// Step B: Compute Input Hash
	hash, err := s.hasher.ComputeInputHash(task, task.Environment, resolvedInputs)
	if err != nil {
		return "", zerr.Wrap(err, "failed to compute input hash")
	}

	return hash, nil
}

// checkTaskCache checks if the task can be skipped based on cached build info.
// Returns skipped (bool), hash (string), and error.
func (s *Scheduler) checkTaskCache(
	_ context.Context,
	task *domain.Task,
	root string,
) (skipped bool, hash string, err error) {
	// Step A: Resolve Inputs
	inputs := make([]string, len(task.Inputs))
	for i, input := range task.Inputs {
		inputs[i] = input.String()
	}
	resolvedInputs, err := s.resolver.ResolveInputs(inputs, root)
	if err != nil {
		return false, "", zerr.Wrap(err, "failed to resolve inputs")
	}

	// Step B: Compute Input Hash
	hash, err = s.hasher.ComputeInputHash(task, task.Environment, resolvedInputs)
	if err != nil {
		return false, "", zerr.Wrap(err, "failed to compute input hash")
	}

	// Step B: Get Build Info from Store
	info, err := s.store.Get(task.Name.String())
	if err != nil {
		return false, hash, zerr.Wrap(err, "failed to get build info")
	}

	// Step C: Compare Hashes
	if info == nil || info.InputHash != hash {
		return false, hash, nil
	}

	// Step D: Verify Outputs
	if !s.verifyOutputsMatch(task, info, root) {
		return false, hash, nil
	}

	return true, hash, nil
}

// verifyOutputsMatch checks if cached outputs match current outputs.
func (s *Scheduler) verifyOutputsMatch(task *domain.Task, info *domain.BuildInfo, root string) bool {
	// Convert InternedString outputs to string slice
	outputs := make([]string, len(task.Outputs))
	for i, out := range task.Outputs {
		outputs[i] = out.String()
	}

	if len(outputs) == 0 {
		return true
	}

	outputHash, err := s.hasher.ComputeOutputHash(outputs, root)
	if err != nil {
		// If error (e.g. file missing), treat as cache miss
		return false
	}

	return info.OutputHash == outputHash
}
