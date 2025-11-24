// Package domain contains the core domain models and business logic for the task dependency graph.
package domain

import (
	"iter"

	"go.trai.ch/zerr"
)

// Graph represents a dependency graph of tasks.
type Graph struct {
	tasks          map[InternedString]Task
	executionOrder []InternedString
}

// NewGraph creates a new empty Graph.
func NewGraph() *Graph {
	return &Graph{
		tasks: make(map[InternedString]Task),
	}
}

// AddTask adds a task to the graph.
// It returns an error if a task with the same name already exists.
func (g *Graph) AddTask(t *Task) error {
	if _, exists := g.tasks[t.Name]; exists {
		return zerr.With(ErrTaskAlreadyExists, "task_name", t.Name.String())
	}
	g.tasks[t.Name] = *t
	return nil
}

// Validate checks for cycles in the graph using a topological sort.
// It populates the executionOrder slice if successful.
func (g *Graph) Validate() error {
	g.executionOrder = make([]InternedString, 0, len(g.tasks))
	visited := make(map[InternedString]int) // 0: unvisited, 1: visiting, 2: visited
	var path []InternedString

	var visit func(u InternedString) error
	visit = func(u InternedString) error {
		visited[u] = 1
		path = append(path, u)

		task, exists := g.tasks[u]
		if !exists {
			return zerr.With(ErrMissingDependency, "dependency", u.String())
		}

		for _, dep := range task.Dependencies {
			if visited[dep] == 1 {
				return g.buildCycleError(path, dep)
			}
			if visited[dep] == 0 {
				if err := visit(dep); err != nil {
					return err
				}
			}
		}

		visited[u] = 2
		path = path[:len(path)-1]
		g.executionOrder = append(g.executionOrder, u)
		return nil
	}

	// We need to iterate over all tasks to ensure we cover disconnected components
	// To ensure deterministic order for disconnected components, we can sort keys or just iterate.
	// Iteration order of map is random. For deterministic build order, we might want to sort keys first.
	// But for correctness, any topological sort is valid.
	// However, usually we want deterministic builds.
	// Let's iterate in a deterministic way (e.g. sorted by name) if we want stability.
	// But for now, random map iteration is acceptable for correctness.
	// Wait, if I want deterministic `Walk`, I should probably iterate sorted keys.
	// I'll stick to map iteration for now as it's O(N). Sorting is O(N log N).
	for name := range g.tasks {
		if visited[name] == 0 {
			if err := visit(name); err != nil {
				return err
			}
		}
	}

	return nil
}

// buildCycleError constructs an error with cycle path metadata.
func (g *Graph) buildCycleError(path []InternedString, dep InternedString) error {
	cyclePath := ""
	startIdx := -1
	for i, node := range path {
		if node == dep {
			startIdx = i
			break
		}
	}
	for i := startIdx; i < len(path); i++ {
		cyclePath += path[i].String() + " -> "
	}
	cyclePath += dep.String()
	return zerr.With(ErrCycleDetected, "cycle", cyclePath)
}

// Walk returns an iterator that yields tasks in execution order.
// It assumes Validate() has been called and returned nil.
func (g *Graph) Walk() iter.Seq[Task] {
	return func(yield func(Task) bool) {
		for _, name := range g.executionOrder {
			if !yield(g.tasks[name]) {
				return
			}
		}
	}
}
