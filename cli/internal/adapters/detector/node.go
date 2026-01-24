package detector

// Node is the graft node for dependency injection.
// The detector has no dependencies.
type Node struct{}

// NewNode creates a new Node.
func NewNode() *Node {
	return &Node{}
}
