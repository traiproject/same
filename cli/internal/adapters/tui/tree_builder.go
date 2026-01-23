package tui

const maxTreeDepth = 10

// buildTree constructs a visual tree from the DAG dependency map.
// Since tasks form a DAG, a task may appear multiple times in the tree
// if it is a dependency of multiple targets.
func buildTree(
	targets []string,
	dependencies map[string][]string,
	taskMap map[string]*TaskNode,
) []*TaskNode {
	roots := make([]*TaskNode, 0, len(targets))

	for _, target := range targets {
		root := buildSubtree(target, dependencies, taskMap, 0)
		if root != nil {
			roots = append(roots, root)
		}
	}

	return roots
}

func buildSubtree(
	taskName string,
	dependencies map[string][]string,
	taskMap map[string]*TaskNode,
	depth int,
) *TaskNode {
	// Guard against very deep trees
	if depth > maxTreeDepth {
		return nil
	}

	// Create a new node instance for each occurrence in the tree
	originalNode := taskMap[taskName]
	if originalNode == nil {
		return nil
	}

	// Clone node for tree position, but keep reference to canonical node
	node := &TaskNode{
		Name:          originalNode.Name,
		Term:          originalNode.Term,
		IsExpanded:    false, // Start collapsed
		Depth:         depth,
		Children:      make([]*TaskNode, 0),
		CanonicalNode: originalNode, // Reference for live status/time updates
	}

	// Recursively build children from dependencies
	deps := dependencies[taskName]
	for _, dep := range deps {
		child := buildSubtree(dep, dependencies, taskMap, depth+1)
		if child != nil {
			child.Parent = node
			node.Children = append(node.Children, child)
		}
	}

	return node
}

// flattenTree converts the tree into a linear list respecting expansion state.
// Only expanded nodes have their children included.
func flattenTree(roots []*TaskNode) []*TaskNode {
	flat := make([]*TaskNode, 0)

	var walk func(node *TaskNode)
	walk = func(node *TaskNode) {
		flat = append(flat, node)
		if node.IsExpanded {
			for _, child := range node.Children {
				walk(child)
			}
		}
	}

	for _, root := range roots {
		walk(root)
	}

	return flat
}
