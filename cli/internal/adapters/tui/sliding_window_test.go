package tui_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"go.trai.ch/same/internal/adapters/telemetry"
	"go.trai.ch/same/internal/adapters/tui"
)

func TestUpdate_SlidingWindow_Scrolling(t *testing.T) {
	// Setup a model with 10 tasks and ListHeight 5
	tasks := make([]*tui.TaskNode, 10)
	taskNames := make([]string, 10)
	for i := 0; i < 10; i++ {
		name := "task" + string(rune('0'+i))
		tasks[i] = &tui.TaskNode{Name: name, Term: tui.NewVterm()}
		taskNames[i] = name
	}

	m := &tui.Model{
		TaskMap:     make(map[string]*tui.TaskNode),
		FlatList:    tasks,
		ListHeight:  5,
		ListOffset:  0,
		SelectedIdx: 0,
		ViewMode:    tui.ViewModeTree,
	}
	for _, task := range tasks {
		m.TaskMap[task.Name] = task
	}

	// 1. Scroll down until the end of the visible window (idx 4)
	// Offset should stay 0
	for i := 0; i < 4; i++ {
		updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = updatedModel.(*tui.Model)
	}
	assert.Equal(t, 4, m.SelectedIdx)
	assert.Equal(t, 0, m.ListOffset)

	// 2. Scroll one more down (idx 5) -> Offset should become 1
	// Window: [1, 2, 3, 4, 5] (indices)
	updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updatedModel.(*tui.Model)
	assert.Equal(t, 5, m.SelectedIdx)
	assert.Equal(t, 1, m.ListOffset)

	// 3. Jump to end (manually verify logic if we could key repeat, but let's just loop)
	for i := 5; i < 9; i++ {
		updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = updatedModel.(*tui.Model)
	}
	assert.Equal(t, 9, m.SelectedIdx)
	// Offset should be: SelectedIdx - ListHeight + 1 = 9 - 5 + 1 = 5
	// Window: [5, 6, 7, 8, 9]
	assert.Equal(t, 5, m.ListOffset)

	// 4. Scroll UP -> Offset should decrease
	// Scroll up to idx 4 -> Offset should become 4
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp}) // idx 8
	m = updatedModel.(*tui.Model)
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp}) // idx 7
	m = updatedModel.(*tui.Model)
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp}) // idx 6
	m = updatedModel.(*tui.Model)
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp}) // idx 5
	m = updatedModel.(*tui.Model)
	// At idx 5, offset is likely still 5 (window 5..9 includes 5)
	assert.Equal(t, 5, m.SelectedIdx)
	assert.Equal(t, 5, m.ListOffset)

	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp}) // idx 4
	m = updatedModel.(*tui.Model)
	assert.Equal(t, 4, m.SelectedIdx)
	// Offset should become 4 to include idx 4
	assert.Equal(t, 4, m.ListOffset)
}

func TestUpdate_SlidingWindow_AutoFollow(t *testing.T) {
	// Setup
	tasks := []*tui.TaskNode{
		{Name: "t0", Term: tui.NewVterm()},
		{Name: "t1", Term: tui.NewVterm()},
		{Name: "t2", Term: tui.NewVterm()},
		{Name: "t3", Term: tui.NewVterm()},
		{Name: "t4", Term: tui.NewVterm()},
		{Name: "t5", Term: tui.NewVterm()},
		{Name: "t6", Term: tui.NewVterm()},
		{Name: "t7", Term: tui.NewVterm()},
		{Name: "t8", Term: tui.NewVterm()},
		{Name: "t9", Term: tui.NewVterm()},
	}
	m := &tui.Model{
		FlatList:   tasks,
		TaskMap:    make(map[string]*tui.TaskNode),
		SpanMap:    make(map[string]*tui.TaskNode),
		ListHeight: 5,
		FollowMode: true,
		ViewMode:   tui.ViewModeTree,
	}
	for _, task := range tasks {
		m.TaskMap[task.Name] = task
	}

	// 1. Task start for t9 -> Should scroll to end
	msg := telemetry.MsgTaskStart{Name: "t9", SpanID: "s9"}
	updatedModel, _ := m.Update(msg)
	m = updatedModel.(*tui.Model)

	assert.Equal(t, 9, m.SelectedIdx)
	assert.Equal(t, 5, m.ListOffset) // 9 - 5 + 1 = 5

	// 2. Task start for t0 -> Should scroll to top
	msg0 := telemetry.MsgTaskStart{Name: "t0", SpanID: "s0"}
	updatedModel, _ = m.Update(msg0)
	m = updatedModel.(*tui.Model)

	assert.Equal(t, 0, m.SelectedIdx)
	assert.Equal(t, 0, m.ListOffset)
}

func TestUpdate_SlidingWindow_Resize(t *testing.T) {
	task := &tui.TaskNode{Name: "t1", Term: tui.NewVterm()}
	m := &tui.Model{
		FlatList: []*tui.TaskNode{task},
		TaskMap:  map[string]*tui.TaskNode{"t1": task},
		ViewMode: tui.ViewModeTree,
	}

	msg := tea.WindowSizeMsg{Width: 100, Height: 50}
	updatedModel, _ := m.Update(msg)
	m = updatedModel.(*tui.Model)

	assert.Less(t, m.ListHeight, 50)
	assert.Greater(t, m.ListHeight, 40)
}
