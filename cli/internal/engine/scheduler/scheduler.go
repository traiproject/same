package scheduler

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"go.trai.ch/same/internal/core/domain"
	"go.trai.ch/same/internal/core/ports"
	"go.trai.ch/zerr"
	"golang.org/x/sync/errgroup"
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
	executor   ports.Executor
	store      ports.BuildInfoStore
	hasher     ports.Hasher
	resolver   ports.InputResolver
	tracer     ports.Tracer
	envFactory ports.EnvironmentFactory

	mu         sync.RWMutex
	taskStatus map[domain.InternedString]TaskStatus
	envCache   sync.Map // map[string][]string - EnvID -> environment variables
}

// NewScheduler creates a new Scheduler with the given dependencies.
func NewScheduler(
	executor ports.Executor,
	store ports.BuildInfoStore,
	hasher ports.Hasher,
	resolver ports.InputResolver,
	tracer ports.Tracer,
	envFactory ports.EnvironmentFactory,
) *Scheduler {
	s := &Scheduler{
		executor:   executor,
		store:      store,
		hasher:     hasher,
		resolver:   resolver,
		tracer:     tracer,
		envFactory: envFactory,
		taskStatus: make(map[domain.InternedString]TaskStatus),
		envCache:   sync.Map{},
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
// Run executes the tasks in the graph with the specified parallelism.
// If targetNames contains "all", all tasks in the graph are executed.
// Otherwise, only the specified tasks are executed.
// If noCache is true, cache is bypassed and all tasks are executed.
func (s *Scheduler) Run(
	ctx context.Context,
	graph *domain.Graph,
	targetNames []string,
	parallelism int,
	noCache bool,
) error {
	// Explicitly validate the graph to ensure executionOrder is populated
	if err := graph.Validate(); err != nil {
		return err
	}

	state, err := s.newRunState(ctx, graph, targetNames, parallelism, noCache)
	if err != nil {
		return err
	}

	// Calculate and emit the build plan based on topological sort
	plannedTasks := make([]string, 0, len(state.allTasks))

	// Create a map for fast lookup of tasks included in this run
	taskSet := make(map[domain.InternedString]bool, len(state.allTasks))
	for _, t := range state.allTasks {
		taskSet[t] = true
	}

	// Filter the graph's full topological order to only include tasks in this run
	// executionOrder is populated by graph.Validate()
	for task := range graph.Walk() {
		if taskSet[task.Name] {
			plannedTasks = append(plannedTasks, task.Name.String())
		}
	}

	// Build dependency map
	depMap := make(map[string][]string)
	for _, taskName := range plannedTasks {
		task, _ := graph.GetTask(domain.NewInternedString(taskName))
		deps := make([]string, len(task.Dependencies))
		for i, dep := range task.Dependencies {
			deps[i] = dep.String()
		}
		depMap[taskName] = deps
	}

	s.tracer.EmitPlan(ctx, plannedTasks, depMap, targetNames)

	// Phase 1: Batch Environment Hydration
	// Resolve all unique environments concurrently before execution starts
	// Wrapped in a span for visibility
	ctx, span := s.tracer.Start(ctx, "Hydrating Environments")
	err = state.prepareEnvironments(ctx)
	span.End()

	if err != nil {
		return err
	}

	s.initTaskStatuses(state.allTasks)

	return state.runExecutionLoop()
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
	noCache     bool
	taskEnvIDs  map[domain.InternedString]string // task name -> environment ID
}

func (s *Scheduler) newRunState(
	ctx context.Context,
	graph *domain.Graph,
	targetNames []string,
	parallelism int,
	noCache bool,
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

	// Pre-calculate environment IDs for all tasks with tools
	taskEnvIDs := make(map[domain.InternedString]string)
	for name := range tasks {
		task := tasks[name]
		if len(task.Tools) > 0 {
			envID := domain.GenerateEnvID(task.Tools)
			taskEnvIDs[name] = envID
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
		noCache:     noCache,
		taskEnvIDs:  taskEnvIDs,
	}, nil
}

func (state *schedulerRunState) runExecutionLoop() error {
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

// prepareEnvironments resolves all required environments concurrently.
func (state *schedulerRunState) prepareEnvironments(ctx context.Context) error {
	// Identify unique environment IDs needed for this run
	neededEnvIDs := make(map[string]map[string]string) // envID -> tools map (sample)

	for taskName, envID := range state.taskEnvIDs {
		if _, exists := neededEnvIDs[envID]; !exists {
			// Find a sample task to get the tools map
			// We can use the current task since it has the correct tools for this EnvID
			if task, ok := state.tasks[taskName]; ok {
				neededEnvIDs[envID] = task.Tools
			}
		}
	}

	// Check which environments are not yet cached
	var envsToResolve []struct {
		id    string
		tools map[string]string
	}

	for id, tools := range neededEnvIDs {
		if _, cached := state.s.envCache.Load(id); !cached {
			envsToResolve = append(envsToResolve, struct {
				id    string
				tools map[string]string
			}{id, tools})
		}
	}

	if len(envsToResolve) == 0 {
		return nil
	}

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(runtime.NumCPU())

	for _, item := range envsToResolve {
		item := item // capture loop var
		g.Go(func() error {
			// Double check cache in case another parallel run hydrated it (optimistic)
			if _, cached := state.s.envCache.Load(item.id); cached {
				return nil
			}

			env, err := state.s.envFactory.GetEnvironment(ctx, item.tools)
			if err != nil {
				return zerr.Wrap(err, "failed to hydrate environment")
			}

			state.s.envCache.Store(item.id, env)
			return nil
		})
	}

	return g.Wait()
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

		t := state.tasks[taskName]
		go state.executeTask(&t)
	}
}

func (state *schedulerRunState) executeTask(t *domain.Task) {
	// Execute the task logic within a function to ensure the span is ended
	// BEFORE we send the result to the channel. This prevents race conditions
	// in tests where the scheduler loop finishes before the span is recorded.
	res := func() result {
		// Start Span
		ctx, span := state.s.tracer.Start(state.ctx, t.Name.String())
		defer span.End()

		// Step 1: Compute Input Hash (Check Cache)
		skipped, hash, err := state.computeInputHash(t)
		if err != nil {
			span.RecordError(err)
			return result{task: t.Name, err: err}
		}

		// If skipped (cached)
		if skipped {
			span.SetAttribute("same.cached", true)
			return result{task: t.Name, skipped: true, inputHash: hash}
		}

		// Step 2: Clean Outputs
		// Clean outputs before building to prevent stale artifacts
		if err = state.validateAndCleanOutputs(t); err != nil {
			span.RecordError(err)
			return result{task: t.Name, err: err}
		}

		// Convert outputs to string slice for result
		outputs := make([]string, len(t.Outputs))
		for i, out := range t.Outputs {
			outputs[i] = out.String()
		}

		// Step 3: Prepare Environment (Phase 1 Hydration)
		var env []string
		if len(t.Tools) > 0 {
			// Environment is already hydrated in Phase 1
			envID := state.taskEnvIDs[t.Name]
			cachedEnv, ok := state.s.envCache.Load(envID)
			if !ok {
				err = zerr.With(domain.ErrEnvironmentNotCached, "env_id", envID)
				span.RecordError(err)
				return result{
					task: t.Name,
					err:  err,
				}
			}
			env = cachedEnv.([]string)
		}

		// Step 4: Execute
		err = state.s.executor.Execute(ctx, t, env, span, span)
		if err != nil {
			span.RecordError(err)
		}

		return result{
			task:        t.Name,
			err:         err,
			inputHash:   hash,
			taskOutputs: outputs,
		}
	}()

	state.resultsCh <- res
}

func (state *schedulerRunState) computeInputHash(t *domain.Task) (skipped bool, hash string, err error) {
	if state.noCache {
		h, forceErr := state.s.computeHashForce(t, state.graph.Root())
		return false, h, forceErr
	}

	// Normal mode: check cache
	skipped, hash, err = state.s.checkTaskCache(state.ctx, t, state.graph.Root())
	if err != nil {
		return false, "", err
	}

	if skipped {
		return true, hash, nil
	}

	return false, hash, nil
}

func (state *schedulerRunState) validateAndCleanOutputs(t *domain.Task) error {
	rootAbs, err := filepath.Abs(state.graph.Root())
	if err != nil {
		return zerr.Wrap(err, domain.ErrFailedToGetRoot.Error())
	}

	for _, out := range t.Outputs {
		outPath := out.String()
		outAbs, err := filepath.Abs(outPath)
		if err != nil {
			return zerr.With(
				zerr.Wrap(err, domain.ErrFailedToGetOutputPath.Error()),
				"file", outPath,
			)
		}

		rel, err := filepath.Rel(rootAbs, outAbs)
		if err != nil {
			return zerr.With(
				zerr.Wrap(err, domain.ErrFailedToResolveRelativePath.Error()),
				"file", outPath,
			)
		}

		if strings.HasPrefix(rel, "..") {
			return zerr.With(
				domain.ErrOutputPathOutsideRoot,
				"file", outPath,
			)
		}

		// Use the validated absolute path for removal to ensure we delete
		// exactly what was validated, preventing potential symlink attacks
		if err := os.RemoveAll(outAbs); err != nil {
			return zerr.With(
				zerr.Wrap(err, domain.ErrFailedToCleanOutput.Error()),
				"file", outPath,
			)
		}
	}

	return nil
}

func (state *schedulerRunState) handleResult(res result) {
	state.active--

	if res.err != nil {
		// Enhance error with task name
		enhancedErr := zerr.With(zerr.Wrap(res.err, domain.ErrTaskExecutionFailed.Error()), "task", res.task.String())
		state.errs = errors.Join(state.errs, enhancedErr)
		state.s.updateStatus(res.task, StatusFailed)
	} else {
		state.handleSuccess(res)
	}
}

func (state *schedulerRunState) handleSuccess(res result) {
	state.s.updateStatus(res.task, StatusCompleted)
	if !res.skipped {
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
				// TODO: Log this error properly once logger is available here or use a specific error metric
				_ = err
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
		return "", zerr.Wrap(err, domain.ErrInputResolutionFailed.Error())
	}

	// Step B: Compute Input Hash
	hash, err := s.hasher.ComputeInputHash(task, task.Environment, resolvedInputs)
	if err != nil {
		return "", zerr.Wrap(err, domain.ErrInputHashComputationFailed.Error())
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
		return false, "", zerr.Wrap(err, domain.ErrInputResolutionFailed.Error())
	}

	// Step B: Compute Input Hash
	hash, err = s.hasher.ComputeInputHash(task, task.Environment, resolvedInputs)
	if err != nil {
		return false, "", zerr.Wrap(err, domain.ErrInputHashComputationFailed.Error())
	}

	// Step B: Get Build Info from Store
	info, err := s.store.Get(task.Name.String())
	if err != nil {
		return false, hash, zerr.Wrap(err, domain.ErrStoreReadFailed.Error())
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
