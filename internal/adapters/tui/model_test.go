package tui_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/bob/internal/adapters/telemetry"
	"go.trai.ch/bob/internal/adapters/tui"
)

func TestWrapLog(t *testing.T) {
	tests := []struct {
		name  string
		input string
		width int
		// Use a function to verify result if simple equality isn't enough (e.g. exact wrapping points)
		verify   func(t *testing.T, input, got string, width int)
		expected string // use strict equality if verify is nil
	}{
		{
			name:  "no wrap needed",
			input: "hello world",
			width: 20,
			verify: func(t *testing.T, input, got string, width int) {
				t.Helper()
				// Expect it to contain the text
				assert.Contains(t, got, input)
				// Check max width of lines
				for _, line := range strings.Split(got, "\n") {
					assert.LessOrEqual(t, len(line), width, "line exceeds width")
				}
			},
		},
		{
			name:  "wrap needed",
			input: "hello world this is a long line",
			width: 10,
			verify: func(t *testing.T, input, got string, width int) {
				t.Helper()
				// Check that we have newlines (it wrapped)
				assert.Contains(t, got, "\n", "should produce newlines")
				// Check max width
				lines := strings.Split(got, "\n")
				for _, line := range lines {
					assert.LessOrEqual(t, len(line), width, "line exceeds width")
				}
				// Verify content is preserved (ignoring whitespace differences caused by wrapping)
				normalizedInput := strings.Join(strings.Fields(input), " ")
				normalizedGot := strings.Join(strings.Fields(got), " ")
				assert.Equal(t, normalizedInput, normalizedGot, "content mismatch")
			},
		},
		{
			name:     "width 0 (safety)",
			input:    "hello world",
			width:    0,
			expected: "hello world",
		},
		{
			name:     "negative width (safety)",
			input:    "hello world",
			width:    -5,
			expected: "hello world",
		},
		{
			name:     "empty input",
			input:    "",
			width:    10,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tui.WrapLog(tt.input, tt.width)
			got = strings.ReplaceAll(got, "\r\n", "\n")

			if tt.verify != nil {
				tt.verify(t, tt.input, got, tt.width)
			} else {
				assert.Equal(t, tt.expected, got)
			}
		})
	}
}

func TestUpdate_InitTasks(t *testing.T) {
	m := &tui.Model{}
	tasks := []string{"task1", "task2", "task3"}
	msg := telemetry.MsgInitTasks{Tasks: tasks}

	updatedModel, cmd := m.Update(msg)
	newM, ok := updatedModel.(*tui.Model)
	require.True(t, ok)

	assert.Nil(t, cmd)
	assert.Len(t, newM.Tasks, len(tasks))
	assert.Len(t, newM.TaskMap, len(tasks))

	for i, task := range tasks {
		assert.Equal(t, task, newM.Tasks[i].Name)
		assert.Equal(t, tui.StatusPending, newM.Tasks[i].Status)
		assert.Same(t, newM.Tasks[i], newM.TaskMap[task])
	}
}

func TestUpdate_Navigation(t *testing.T) {
	// Initialize directly with tasks
	m := &tui.Model{
		Tasks: []*tui.TaskNode{
			{Name: "task1"},
			{Name: "task2"},
			{Name: "task3"},
		},
		SelectedIdx: 0,
		FollowMode:  true,
		TaskMap:     make(map[string]*tui.TaskNode),
	}
	// Setup map pointers
	for i := range m.Tasks {
		m.TaskMap[m.Tasks[i].Name] = m.Tasks[i]
	}

	// Case 1: Down
	// tea.KeyMsg matching "down"
	msgDown := tea.KeyMsg{Type: tea.KeyDown}

	updatedModel, _ := m.Update(msgDown)
	newM, ok := updatedModel.(*tui.Model)
	require.True(t, ok)

	assert.Equal(t, 1, newM.SelectedIdx)
	assert.False(t, newM.FollowMode, "FollowMode should be false after navigation")

	// Case 2: Up
	msgUp := tea.KeyMsg{Type: tea.KeyUp}
	updatedModel, _ = newM.Update(msgUp)
	newM, ok = updatedModel.(*tui.Model)
	require.True(t, ok)

	assert.Equal(t, 0, newM.SelectedIdx)
	assert.False(t, newM.FollowMode)

	// Case 3: Activate Follow Mode (Escape)
	newM.FollowMode = false
	// Start a task to see if it jumps to running (as per logic: "jump to the currently running task if any")
	newM.Tasks[2].Status = tui.StatusRunning

	msgEsc := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, _ = newM.Update(msgEsc)
	newM, ok = updatedModel.(*tui.Model)
	require.True(t, ok)

	assert.True(t, newM.FollowMode)
	assert.Equal(t, 2, newM.SelectedIdx, "Should jump to running task index")
}

func TestUpdate_AutoFollow(t *testing.T) {
	// Setup
	tasks := []string{"task1", "task2", "task3"}
	m := &tui.Model{}
	updatedModel, _ := m.Update(telemetry.MsgInitTasks{Tasks: tasks})
	m = updatedModel.(*tui.Model)

	// Case 1: Follow Mode True
	m.FollowMode = true
	m.SelectedIdx = 0
	m.ActiveTaskName = "task1"

	// Send Start for task2
	msg := telemetry.MsgTaskStart{Name: "task2", SpanID: "span2"}
	updatedModel, _ = m.Update(msg)
	newM, ok := updatedModel.(*tui.Model)
	require.True(t, ok)

	assert.Equal(t, "task2", newM.ActiveTaskName, "Should switch active task in follow mode")
	assert.Equal(t, 1, newM.SelectedIdx)
	assert.Equal(t, tui.StatusRunning, newM.TaskMap["task2"].Status)

	// Case 2: Follow Mode False
	newM.FollowMode = false
	// Simulate user selected task3 manually
	newM.SelectedIdx = 2
	newM.ActiveTaskName = "task3"

	// Send Start for task1
	msgStart := telemetry.MsgTaskStart{Name: "task1", SpanID: "span1"}
	updatedModel2, _ := newM.Update(msgStart)
	newM2, ok := updatedModel2.(*tui.Model)
	require.True(t, ok)

	// Active task name should stay as "task3" because we are NOT following
	assert.Equal(t, "task3", newM2.ActiveTaskName, "Should NOT switch active task when not in follow mode")
	// SelectedIdx should NOT change
	assert.Equal(t, 2, newM2.SelectedIdx)
	// But status of task1 SHOULD update
	assert.Equal(t, tui.StatusRunning, newM2.TaskMap["task1"].Status)
}

func TestUpdate_Logs(t *testing.T) {
	// Setup
	m := &tui.Model{
		Tasks: []*tui.TaskNode{{Name: "task1", Status: tui.StatusRunning}},
		Viewport: viewport.Model{
			Width:  100,
			Height: 20,
		},
		ActiveTaskName: "task1",
		AutoScroll:     true,
		TaskMap:        make(map[string]*tui.TaskNode),
		SpanMap:        make(map[string]*tui.TaskNode),
	}
	m.TaskMap["task1"] = m.Tasks[0]
	m.SpanMap["span1"] = m.Tasks[0] // associate span1 with task1

	// Send Log
	logData := []byte("hello world")
	msg := telemetry.MsgTaskLog{SpanID: "span1", Data: logData}

	updatedModel, cmd := m.Update(msg)
	newM, ok := updatedModel.(*tui.Model)
	require.True(t, ok)

	assert.Nil(t, cmd)
	assert.Equal(t, logData, newM.Tasks[0].Logs)
	assert.Contains(t, newM.Viewport.View(), "hello world")
}

func TestUpdate_WindowSize(t *testing.T) {
	m := &tui.Model{
		Tasks:          []*tui.TaskNode{{Name: "task1", Logs: []byte("some logs")}},
		ActiveTaskName: "task1",
		TaskMap:        make(map[string]*tui.TaskNode),
	}
	m.TaskMap["task1"] = m.Tasks[0]
	m.Viewport.Width = 10
	m.Viewport.Height = 10

	// Send WindowSizeMsg
	// Width 100 -> List gets 30, Logs get 100 - 30 - 4 = 66
	msg := tea.WindowSizeMsg{Width: 100, Height: 50}

	updatedModel, _ := m.Update(msg)
	newM, ok := updatedModel.(*tui.Model)
	require.True(t, ok)

	// Check dimensions
	// listWidthRatio = 0.3
	assert.Equal(t, 48, newM.Viewport.Height) // 50 - 2
	assert.Equal(t, 66, newM.Viewport.Width)
}

func TestUpdate_TaskComplete(t *testing.T) {
	m := &tui.Model{
		Tasks:   []*tui.TaskNode{{Name: "task1", Status: tui.StatusRunning}},
		SpanMap: make(map[string]*tui.TaskNode),
	}
	m.SpanMap["span1"] = m.Tasks[0]

	// Success case
	msgSuccess := telemetry.MsgTaskComplete{SpanID: "span1", Err: nil}
	updatedModel, _ := m.Update(msgSuccess)
	newM, ok := updatedModel.(*tui.Model)
	require.True(t, ok)
	assert.Equal(t, tui.StatusDone, newM.Tasks[0].Status)

	// Error case
	// Reset status
	m.Tasks[0].Status = tui.StatusRunning
	msgError := telemetry.MsgTaskComplete{SpanID: "span1", Err: assert.AnError}
	updatedModel, _ = m.Update(msgError)
	newM, ok = updatedModel.(*tui.Model)
	require.True(t, ok)
	assert.Equal(t, tui.StatusError, newM.Tasks[0].Status)
}

func TestInit(t *testing.T) {
	m := &tui.Model{}
	cmd := m.Init()
	assert.Nil(t, cmd)
}

func TestUpdate_Quit(t *testing.T) {
	m := &tui.Model{}

	// Test "q"
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	assert.Equal(t, tea.Quit(), cmd())

	// Test "ctrl+c"
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	assert.Equal(t, tea.Quit(), cmd())
}

func TestUpdate_Logs_Truncation(t *testing.T) {
	m := &tui.Model{
		Tasks:   []*tui.TaskNode{{Name: "task1"}},
		SpanMap: make(map[string]*tui.TaskNode),
	}
	m.SpanMap["span1"] = m.Tasks[0]

	// Create a log message slightly larger than 1MB
	// 1MB = 1024 * 1024 = 1048576 bytes
	largeData := make([]byte, 1024*1024+100)
	for i := range largeData {
		largeData[i] = 'a'
	}
	msg := telemetry.MsgTaskLog{SpanID: "span1", Data: largeData}

	updatedModel, _ := m.Update(msg)
	newM, ok := updatedModel.(*tui.Model)
	require.True(t, ok)

	// Logs should be truncated to exactly 1MB
	assert.Len(t, newM.Tasks[0].Logs, 1024*1024)
}

func TestUpdate_Logs_Truncation_Safe(t *testing.T) {
	m := &tui.Model{
		Tasks:   []*tui.TaskNode{{Name: "task1"}},
		SpanMap: make(map[string]*tui.TaskNode),
	}
	m.SpanMap["span1"] = m.Tasks[0]

	// 1. Test truncation at newline
	// Log = "prefix..." (target cut point) + "keep\n" + "survive"
	// We want to make sure we cut BEFORE "survive" but AFTER "keep\n" IF the cut point falls in "keep".
	// Actually, the requirement is "start index for the cut ... scan forward to find the first newline".

	// Let's construct a scenario:
	// MaxLogSize is small for testing purposes?
	// The implementation uses a const maxLogSize = 1024 * 1024.
	// We cannot easily change that constant for testing effectively without exporting it or refactoring.
	// However, we can test that it DOES NOT split a UTF-8 character if we happen to land on one.

	// Create a buffer slightly larger than 1MB.
	// 1MB = 1048576 bytes.
	// We will fill it such that the split point (len - maxLogSize) lands right in the middle of a multi-byte rune.

	const maxLogSize = 1024 * 1024
	extraBytes := 10 // We are 10 bytes over the limit.
	totalSize := maxLogSize + extraBytes

	// Construct data:
	// We want the cut point `start = totalSize - maxLogSize = 10`.
	// So we want index 10 to be in the middle of a rune.
	// Let's place a multi-byte rune at index 9 (length 3, e.g., '⌘' - 0xE2 0x8C 0x98).
	// Index 9: 0xE2
	// Index 10: 0x8C (Cut point!)
	// Index 11: 0x98

	data := make([]byte, totalSize)
	// Fill with 'a'
	for i := range data {
		data[i] = 'a'
	}

	// Insert the rune at index 9
	// '⌘' (Place of Interest Sign) is 3 bytes: E2 8C 98
	data[9] = 0xE2
	data[10] = 0x8C
	data[11] = 0x98

	// So naive cut at index 10 (keeping last maxLogSize) means:
	// data[10:] -> starts with 0x8C -> invalid utf8.

	msg := telemetry.MsgTaskLog{SpanID: "span1", Data: data}

	updatedModel, _ := m.Update(msg)
	newM, ok := updatedModel.(*tui.Model)
	require.True(t, ok)

	logs := newM.Tasks[0].Logs
	// Check that we didn't produce invalid UTF-8 at the start
	assert.True(t, utf8.Valid(logs), "Logs should contain valid UTF-8 sequence")

	// The logic should have advanced to start of next rune or found a newline.
	// We didn't put newlines, so it should rely on UTF-8 check.
	// It should retain from index 12 ('a's start) or if it handles the rune correctly, it might drop the whole rune.
	// If it safely cuts, it must start with valid rune.

	// 2. Test truncation at newline priority
	// Create content: "garbage\nuseful content..." where the cut point is in "garbage".
	// data size = maxLogSize + 20. Cut point = 20.
	// "01234567890123456789\nKeeping this"
	//                      ^ index 20 (newline is at 20)

	data2 := make([]byte, maxLogSize+20)
	copy(data2, "0123456789012345678\n") // 20 bytes: 0..18 digits, 19 is newline.
	// rest 'b'
	for i := 20; i < len(data2); i++ {
		data2[i] = 'b'
	}

	// Cut point is index 20.
	// If we have "something\n" closer, it's good.
	// Let's make the newline happen AFTER the naive cut point to force a scan forward.
	// Cut point = 20.
	// "0....(25 bytes)....24\nKeep"
	// Naive cut at 20. Newline at 25. Should scan to 25 and cut after.

	data3 := make([]byte, maxLogSize+30)
	// first 30 bytes
	prefix := []byte("0123456789012345678901234\nsucc") // \n at index 25
	copy(data3, prefix)

	// Naive cut should be at index 30.
	// Wait, len=maxLogSize+30. Naive keep = maxLogSize. Start index = 30.
	// If we want to test scanning FORWARD, the newline must be >= 30.
	// "0....(35 chars)....\nKeep" -> Cut at 30. Scan finds \n at 36. Keep from 37.

	prefixLong := []byte("012345678901234567890123456789012345\n") // \n at 36
	copy(data3, prefixLong)

	m2 := &tui.Model{
		Tasks:   []*tui.TaskNode{{Name: "task2"}},
		SpanMap: make(map[string]*tui.TaskNode),
	}
	m2.SpanMap["span2"] = m2.Tasks[0]

	msg2 := telemetry.MsgTaskLog{SpanID: "span2", Data: data3}
	updatedModel2, _ := m2.Update(msg2)
	require.NotNil(t, updatedModel2)

	// It should have cut AFTER the newline at 36.
	// So logs should start with whatever was after \n
	// (which is 0 in our init buffer, so empty/nulls in this case, or let's fill it)
	// Let's actually fill data3 with verifiable content
	for i := len(prefixLong); i < len(data3); i++ {
		data3[i] = 'x'
	}

	msg3 := telemetry.MsgTaskLog{SpanID: "span2", Data: data3}
	// Re-init m2 to clear logs
	m2.Tasks[0].Logs = nil
	updatedModel3, _ := m2.Update(msg3)
	newM3, ok := updatedModel3.(*tui.Model)
	require.True(t, ok)

	logStr := string(newM3.Tasks[0].Logs)
	// Should start with 'x'
	assert.True(t, strings.HasPrefix(logStr, "x"), "Should start with content after newline, got: %q", logStr[:10])
}

func TestUpdate_Logs_InactiveTask(t *testing.T) {
	m := &tui.Model{
		Tasks:          []*tui.TaskNode{{Name: "task1"}, {Name: "task2"}},
		ActiveTaskName: "task1",
		SpanMap:        make(map[string]*tui.TaskNode),
		Viewport:       viewport.New(100, 20),
	}
	m.SpanMap["span2"] = m.Tasks[1] // associate span2 with task2 (inactive)

	msg := telemetry.MsgTaskLog{SpanID: "span2", Data: []byte("log for task 2")}

	updatedModel, _ := m.Update(msg)
	newM, ok := updatedModel.(*tui.Model)
	require.True(t, ok)

	// Logs for task2 should be updated
	assert.Equal(t, []byte("log for task 2"), newM.Tasks[1].Logs)
	// Viewport should NOT be updated (still showing task1 empty logs)
	assert.NotContains(t, newM.Viewport.View(), "log for task 2")
}

func TestUpdate_Logs_NoAutoScroll(t *testing.T) {
	m := &tui.Model{
		Tasks:          []*tui.TaskNode{{Name: "task1"}},
		ActiveTaskName: "task1",
		AutoScroll:     false,
		SpanMap:        make(map[string]*tui.TaskNode),
		Viewport:       viewport.New(100, 20),
	}
	m.SpanMap["span1"] = m.Tasks[0]

	// Pre-fill to ensure we have scrolling capability if we wanted
	m.Tasks[0].Logs = []byte("line1\nline2\n")
	m.Viewport.SetContent("line1\nline2\n")
	m.Viewport.SetYOffset(0)

	msg := telemetry.MsgTaskLog{SpanID: "span1", Data: []byte("line3\n")}

	updatedModel, _ := m.Update(msg)
	newM, ok := updatedModel.(*tui.Model)
	require.True(t, ok)

	// Logs updated
	assert.Contains(t, string(newM.Tasks[0].Logs), "line3")
	// Viewport content updated
	assert.Contains(t, newM.Viewport.View(), "line3")
	// But YOffset should logic implies we probably haven't explicitly scrolled to bottom?
	// Viewport.GotoBottom() sets YOffset.
	// We can check if GotoBottom was called by checking AtBottom(), but that depends on height.
	// Since we didn't scroll, we might not be at bottom if content > height.
	// Let's create enough content to scroll.

	// New attempt with heavy content
	longContent := strings.Repeat("line\n", 50) // > 20 lines
	m.Tasks[0].Logs = []byte(longContent)
	m.Viewport.SetContent(longContent)
	m.Viewport.SetYOffset(0) // Explicitly at top

	msg2 := telemetry.MsgTaskLog{SpanID: "span1", Data: []byte("newline\n")}
	updatedModel2, _ := m.Update(msg2)
	newM2, ok := updatedModel2.(*tui.Model)
	require.True(t, ok)

	// Should still be at top (offset 0) because AutoScroll is false
	assert.Equal(t, 0, newM2.Viewport.YOffset)
}

func TestUpdate_EmptyTasks_Esc(t *testing.T) {
	m := &tui.Model{
		Tasks: []*tui.TaskNode{},
	}

	// Press Esc with empty tasks
	updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	newM, ok := updatedModel.(*tui.Model)
	require.True(t, ok)

	// Should not panic, FollowMode should be true
	assert.True(t, newM.FollowMode)
}

func TestUpdate_Defensive_UnknownItems(t *testing.T) {
	m := &tui.Model{
		Tasks:          []*tui.TaskNode{{Name: "task1"}},
		TaskMap:        make(map[string]*tui.TaskNode),
		SpanMap:        make(map[string]*tui.TaskNode),
		ActiveTaskName: "ghost_task",
		Viewport:       viewport.New(10, 10),
	}
	// Note: ghost_task is NOT in TaskMap

	// Case 1: WindowSizeMsg with active task name that doesn't exist in map
	msgSize := tea.WindowSizeMsg{Width: 100, Height: 50}
	updatedModel, _ := m.Update(msgSize)
	// Should not panic
	require.NotNil(t, updatedModel)

	// Case 2: TaskStart for unknown task
	msgStart := telemetry.MsgTaskStart{Name: "unknown", SpanID: "spanX"}
	updatedModel, _ = m.Update(msgStart)
	// Should do nothing (no panic)
	require.NotNil(t, updatedModel)

	// Case 3: TaskLog for unknown span
	msgLog := telemetry.MsgTaskLog{SpanID: "unknown_span", Data: []byte("foo")}
	updatedModel, _ = m.Update(msgLog)
	require.NotNil(t, updatedModel)

	// Case 4: TaskComplete for unknown span
	msgComplete := telemetry.MsgTaskComplete{SpanID: "unknown_span", Err: nil}
	updatedModel, _ = m.Update(msgComplete)
	require.NotNil(t, updatedModel)
}
