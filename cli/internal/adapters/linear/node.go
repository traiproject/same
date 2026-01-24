package linear

// Node is the graft node for dependency injection.
// The linear renderer has no dependencies.
type Node struct{}

// NewNode creates a new Node.
func NewNode() *Node {
	return &Node{}
}

// Renderer returns a new LinearRenderer with stdout and stderr.
func (n *Node) Renderer() *Renderer {
	return NewRenderer(nil, nil)
}
