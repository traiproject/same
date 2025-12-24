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
