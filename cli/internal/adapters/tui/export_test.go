package tui

// Export functions for testing
var (
	BuildTree   = buildTree
	FlattenTree = flattenTree
)

// MaxOffset exposes the private maxOffset method for testing.
func (v *Vterm) MaxOffset() int {
	return v.maxOffset()
}
