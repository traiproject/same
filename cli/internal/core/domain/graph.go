// Package domain contains the core domain models and business logic for the task dependency graph.
package domain

import (
	"iter"
	"slices"

	"go.trai.ch/zerr"
)

// Graph represents a dependency graph of tasks.
type Graph struct {
	tasks          map[InternedString]Task
	executionOrder []InternedString
	dependents     map[InternedString][]InternedString
	root           string
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
// It populates the executionOrder slice and dependents map if successful.
func (g *Graph) Validate() error {
	g.executionOrder = make([]InternedString, 0, len(g.tasks))
	g.dependents = g.buildDependentsMap()
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

	// We need to iterate over all tasks to ensure we cover disconnected components.
	// To ensure deterministic order for disconnected components, we sort the keys alphabetically.
	sortedNames := g.getSortedTaskNames()

	for _, name := range sortedNames {
		if visited[name] == 0 {
			if err := visit(name); err != nil {
				return err
			}
		}
	}

	return nil
}

// buildDependentsMap creates a reverse adjacency list (dependents map).
func (g *Graph) buildDependentsMap() map[InternedString][]InternedString {
	dependents := make(map[InternedString][]InternedString)
	for taskName := range g.tasks {
		task := g.tasks[taskName]
		for _, dep := range task.Dependencies {
			dependents[dep] = append(dependents[dep], task.Name)
		}
	}
	return dependents
}

// getSortedTaskNames returns all task names sorted alphabetically.
func (g *Graph) getSortedTaskNames() []InternedString {
	sortedNames := make([]InternedString, 0, len(g.tasks))
	for name := range g.tasks {
		sortedNames = append(sortedNames, name)
	}
	slices.SortFunc(sortedNames, func(a, b InternedString) int {
		if a.String() < b.String() {
			return -1
		}
		if a.String() > b.String() {
			return 1
		}
		return 0
	})
	return sortedNames
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

// Dependents returns the list of tasks that depend on the given task.
// Returns an empty slice if no tasks depend on it.
func (g *Graph) Dependents(task InternedString) []InternedString {
	return g.dependents[task]
}

// TaskCount returns the total number of tasks in the graph.
func (g *Graph) TaskCount() int {
	return len(g.tasks)
}

// GetTask retrieves a task by its name.
func (g *Graph) GetTask(name InternedString) (Task, bool) {
	t, ok := g.tasks[name]
	return t, ok
}

// Root returns the root directory of the build.
func (g *Graph) Root() string {
	return g.root
}

// SetRoot sets the root directory of the build.
func (g *Graph) SetRoot(path string) {
	g.root = path
}
