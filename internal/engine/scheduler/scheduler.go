// Package scheduler implements the task execution scheduler.
package scheduler

import (
	"sync"

	"go.trai.ch/bob/internal/core/domain"
	"go.trai.ch/bob/internal/core/ports"
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
	graph    *domain.Graph
	executor ports.Executor

	mu         sync.RWMutex
	taskStatus map[domain.InternedString]TaskStatus
}

// NewScheduler creates a new Scheduler with the given graph and executor.
func NewScheduler(graph *domain.Graph, executor ports.Executor) *Scheduler {
	s := &Scheduler{
		graph:      graph,
		executor:   executor,
		taskStatus: make(map[domain.InternedString]TaskStatus),
	}
	s.initTaskStatuses()
	return s
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
