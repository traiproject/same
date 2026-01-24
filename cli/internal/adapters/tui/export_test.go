package tui

var (
	BuildTree   = buildTree
	FlattenTree = flattenTree
)

func (v *Vterm) MaxOffset() int {
	return v.maxOffset()
}

func (m *Model) GetSelectedTask() *TaskNode {
	return m.getSelectedTask()
}

func (m *Model) UpdateActiveView() {
	m.updateActiveView()
}

func (m *Model) EnsureVisible() {
	m.ensureVisible()
}

func CalculateRowNameWidth(node *TaskNode) int {
	return calculateRowNameWidth(node)
}

func (m *Model) CalculateMaxNameWidth(start, end int) int {
	return m.calculateMaxNameWidth(start, end)
}
