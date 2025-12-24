//nolint:testpackage // Test needs access to unexported fields
package tui

import (
	"strings"
	"testing"
)

func TestModel_View(t *testing.T) {
	m := NewModel(nil)
	m.width = 80
	m.height = 20

	m.vertices = []VertexState{
		{ID: "1", Name: "Root Task", Status: statusRunning, IndentationLevel: 0},
		{ID: "2", Name: "Child Task", Status: statusCompleted, IndentationLevel: 1, ParentID: "1"},
		{ID: "3", Name: "Failed Task", Status: statusFailed, IndentationLevel: 0},
	}

	output := m.View()

	t.Logf("View Output:\n%s", output)

	// Check content presence
	if !strings.Contains(output, "Root Task") {
		t.Errorf("Expected output to contain 'Root Task'")
	}
	if !strings.Contains(output, "Child Task") {
		t.Errorf("Expected output to contain 'Child Task'")
	}
	if !strings.Contains(output, "Failed Task") {
		t.Errorf("Expected output to contain 'Failed Task'")
	}

	// Check for icons (approximate since they are colored)
	// Completed has "✓"
	if !strings.Contains(output, "✓") {
		t.Errorf("Expected output to contain checkmark for completed task")
	}
	// Failed has "✗"
	if !strings.Contains(output, "✗") {
		t.Errorf("Expected output to contain cross for failed task")
	}
}

func TestModel_View_ExpandedLogs(t *testing.T) {
	m := NewModel(nil)
	m.width = 80
	m.height = 20

	vID := "task-1"
	m.vertices = []VertexState{
		{ID: vID, Name: "Task with Logs", Status: statusRunning, Expanded: true},
	}

	m.logs[vID] = []string{
		"Log line 1",
		"Log line 2",
		"Log line 3",
	}

	output := m.View()
	t.Logf("View Output with Logs:\n%s", output)

	// Check for logs
	if !strings.Contains(output, "Log line 1") {
		t.Errorf("Expected output to contain 'Log line 1'")
	}
	if !strings.Contains(output, "Log line 3") {
		t.Errorf("Expected output to contain 'Log line 3'")
	}

	// Verify tailing: Add more logs than limit (5)
	m.logs[vID] = []string{"1", "2", "3", "4", "5", "6", "7"}
	output = m.View()
	if strings.Contains(output, "1") {
		t.Error("Expected log '1' to be truncated")
	}
	if !strings.Contains(output, "7") {
		t.Error("Expected log '7' to be present")
	}

	m.logs[vID] = []string{
		"older_log",
		"log_A", "log_B", "log_C", "log_D", "log_E",
	}
	// limit is 5, so "older_log" should be gone

	output = m.View()
	if strings.Contains(output, "older_log") {
		t.Error("Expected older log to be truncated")
	}
	if !strings.Contains(output, "log_E") {
		t.Error("Expected new log to be present")
	}
}
