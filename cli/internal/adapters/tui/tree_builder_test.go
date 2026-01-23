package tui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.trai.ch/same/internal/adapters/tui"
)

func TestBuildTree_SimpleDependency(t *testing.T) {
	t.Parallel()

	taskMap := map[string]*tui.TaskNode{
		"A": {Name: "A", Term: tui.NewVterm()},
		"B": {Name: "B", Term: tui.NewVterm()},
		"C": {Name: "C", Term: tui.NewVterm()},
	}

	dependencies := map[string][]string{
		"A": {"B"},
		"B": {"C"},
		"C": {},
	}

	targets := []string{"A"}

	roots := tui.BuildTree(targets, dependencies, taskMap)

	assert.Len(t, roots, 1)
	assert.Equal(t, "A", roots[0].Name)
	assert.Len(t, roots[0].Children, 1)
	assert.Equal(t, "B", roots[0].Children[0].Name)
	assert.Len(t, roots[0].Children[0].Children, 1)
	assert.Equal(t, "C", roots[0].Children[0].Children[0].Name)

	// Verify canonical node references are set
	assert.NotNil(t, roots[0].CanonicalNode)
	assert.Equal(t, taskMap["A"], roots[0].CanonicalNode)
	assert.NotNil(t, roots[0].Children[0].CanonicalNode)
	assert.Equal(t, taskMap["B"], roots[0].Children[0].CanonicalNode)
}

func TestBuildTree_StatusUpdates(t *testing.T) {
	t.Parallel()

	// Create canonical nodes
	taskMap := map[string]*tui.TaskNode{
		"A": {Name: "A", Term: tui.NewVterm(), Status: tui.StatusPending},
		"B": {Name: "B", Term: tui.NewVterm(), Status: tui.StatusPending},
	}

	dependencies := map[string][]string{
		"A": {"B"},
		"B": {},
	}

	targets := []string{"A"}

	roots := tui.BuildTree(targets, dependencies, taskMap)

	// Initially both should reference pending status
	assert.Equal(t, tui.StatusPending, roots[0].CanonicalNode.Status)
	assert.Equal(t, tui.StatusPending, roots[0].Children[0].CanonicalNode.Status)

	// Update canonical node status
	taskMap["A"].Status = tui.StatusRunning
	taskMap["B"].Status = tui.StatusDone

	// Tree nodes should reflect updated status via canonical reference
	assert.Equal(t, tui.StatusRunning, roots[0].CanonicalNode.Status)
	assert.Equal(t, tui.StatusDone, roots[0].Children[0].CanonicalNode.Status)
}

func TestBuildTree_SharedDependency(t *testing.T) {
	t.Parallel()

	// Diamond: A -> B -> D, A -> C -> D
	// D appears twice in the tree

	taskMap := map[string]*tui.TaskNode{
		"A": {Name: "A", Term: tui.NewVterm()},
		"B": {Name: "B", Term: tui.NewVterm()},
		"C": {Name: "C", Term: tui.NewVterm()},
		"D": {Name: "D", Term: tui.NewVterm()},
	}

	dependencies := map[string][]string{
		"A": {"B", "C"},
		"B": {"D"},
		"C": {"D"},
		"D": {},
	}

	targets := []string{"A"}

	roots := tui.BuildTree(targets, dependencies, taskMap)

	assert.Len(t, roots, 1)
	assert.Equal(t, "A", roots[0].Name)
	assert.Len(t, roots[0].Children, 2)

	// Verify D appears under both B and C
	bNode := roots[0].Children[0]
	cNode := roots[0].Children[1]

	assert.Len(t, bNode.Children, 1)
	assert.Equal(t, "D", bNode.Children[0].Name)

	assert.Len(t, cNode.Children, 1)
	assert.Equal(t, "D", cNode.Children[0].Name)
}

func TestBuildTree_NoDependencies(t *testing.T) {
	t.Parallel()

	taskMap := map[string]*tui.TaskNode{
		"A": {Name: "A", Term: tui.NewVterm()},
		"B": {Name: "B", Term: tui.NewVterm()},
	}

	dependencies := map[string][]string{
		"A": {},
		"B": {},
	}

	targets := []string{"A", "B"}

	roots := tui.BuildTree(targets, dependencies, taskMap)

	assert.Len(t, roots, 2)
	assert.Equal(t, "A", roots[0].Name)
	assert.Equal(t, "B", roots[1].Name)
	assert.Empty(t, roots[0].Children)
	assert.Empty(t, roots[1].Children)
}

func TestBuildTree_MultipleTargets(t *testing.T) {
	t.Parallel()

	taskMap := map[string]*tui.TaskNode{
		"A": {Name: "A", Term: tui.NewVterm()},
		"B": {Name: "B", Term: tui.NewVterm()},
		"C": {Name: "C", Term: tui.NewVterm()},
	}

	dependencies := map[string][]string{
		"A": {"C"},
		"B": {"C"},
		"C": {},
	}

	targets := []string{"A", "B"}

	roots := tui.BuildTree(targets, dependencies, taskMap)

	assert.Len(t, roots, 2)
	assert.Equal(t, "A", roots[0].Name)
	assert.Equal(t, "B", roots[1].Name)

	// C appears under both A and B
	assert.Len(t, roots[0].Children, 1)
	assert.Equal(t, "C", roots[0].Children[0].Name)

	assert.Len(t, roots[1].Children, 1)
	assert.Equal(t, "C", roots[1].Children[0].Name)
}

func TestFlattenTree_Collapsed(t *testing.T) {
	t.Parallel()

	parent := &tui.TaskNode{
		Name:       "parent",
		IsExpanded: false,
		Children: []*tui.TaskNode{
			{Name: "child1", Term: tui.NewVterm()},
			{Name: "child2", Term: tui.NewVterm()},
		},
		Term: tui.NewVterm(),
	}

	roots := []*tui.TaskNode{parent}
	flat := tui.FlattenTree(roots)

	// Only parent should be in flat list since it's collapsed
	assert.Len(t, flat, 1)
	assert.Equal(t, "parent", flat[0].Name)
}

func TestFlattenTree_Expanded(t *testing.T) {
	t.Parallel()

	child1 := &tui.TaskNode{Name: "child1", Term: tui.NewVterm()}
	child2 := &tui.TaskNode{Name: "child2", Term: tui.NewVterm()}

	parent := &tui.TaskNode{
		Name:       "parent",
		IsExpanded: true,
		Children:   []*tui.TaskNode{child1, child2},
		Term:       tui.NewVterm(),
	}

	roots := []*tui.TaskNode{parent}
	flat := tui.FlattenTree(roots)

	// All nodes should be in flat list
	assert.Len(t, flat, 3)
	assert.Equal(t, "parent", flat[0].Name)
	assert.Equal(t, "child1", flat[1].Name)
	assert.Equal(t, "child2", flat[2].Name)
}

func TestFlattenTree_NestedExpansion(t *testing.T) {
	t.Parallel()

	grandchild := &tui.TaskNode{Name: "grandchild", Term: tui.NewVterm()}

	child := &tui.TaskNode{
		Name:       "child",
		IsExpanded: true,
		Children:   []*tui.TaskNode{grandchild},
		Term:       tui.NewVterm(),
	}

	parent := &tui.TaskNode{
		Name:       "parent",
		IsExpanded: true,
		Children:   []*tui.TaskNode{child},
		Term:       tui.NewVterm(),
	}

	roots := []*tui.TaskNode{parent}
	flat := tui.FlattenTree(roots)

	// All three levels should be visible
	assert.Len(t, flat, 3)
	assert.Equal(t, "parent", flat[0].Name)
	assert.Equal(t, "child", flat[1].Name)
	assert.Equal(t, "grandchild", flat[2].Name)
}

func TestFlattenTree_PartialExpansion(t *testing.T) {
	t.Parallel()

	grandchild := &tui.TaskNode{Name: "grandchild", Term: tui.NewVterm()}

	child := &tui.TaskNode{
		Name:       "child",
		IsExpanded: false, // Collapsed
		Children:   []*tui.TaskNode{grandchild},
		Term:       tui.NewVterm(),
	}

	parent := &tui.TaskNode{
		Name:       "parent",
		IsExpanded: true,
		Children:   []*tui.TaskNode{child},
		Term:       tui.NewVterm(),
	}

	roots := []*tui.TaskNode{parent}
	flat := tui.FlattenTree(roots)

	// Grandchild should not be visible since child is collapsed
	assert.Len(t, flat, 2)
	assert.Equal(t, "parent", flat[0].Name)
	assert.Equal(t, "child", flat[1].Name)
}
