package tui

// MaxOffset exposes the private maxOffset method for testing.
func (v *Vterm) MaxOffset() int {
	return v.maxOffset()
}
