package linear_test

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"go.trai.ch/same/internal/adapters/linear"
	"go.trai.ch/zerr"
)

func TestRenderer_TaskLifecycle(t *testing.T) {
	var stdout, stderr bytes.Buffer
	r := linear.NewRenderer(&stdout, &stderr)

	ctx := context.Background()
	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Plan
	r.OnPlanEmit([]string{"task1", "task2"}, map[string][]string{
		"task2": {"task1"},
	}, []string{"task2"})

	if !strings.Contains(stderr.String(), "Planning to build 2 task(s)") {
		t.Errorf("Expected plan message in stderr, got: %s", stderr.String())
	}

	// Task start
	startTime := time.Now()
	r.OnTaskStart("span1", "", "task1", startTime)

	if !strings.Contains(stderr.String(), "[task1]") {
		t.Errorf("Expected task start message, got: %s", stderr.String())
	}

	// Task logs
	r.OnTaskLog("span1", []byte("first line\n"))
	r.OnTaskLog("span1", []byte("second line\n"))

	stdoutStr := stdout.String()
	if !strings.Contains(stdoutStr, "task1") || !strings.Contains(stdoutStr, "first line") {
		t.Errorf("Expected prefixed first line in stdout, got: %s", stdoutStr)
	}
	if !strings.Contains(stdoutStr, "task1") || !strings.Contains(stdoutStr, "second line") {
		t.Errorf("Expected prefixed second line in stdout, got: %s", stdoutStr)
	}

	// Task complete
	endTime := startTime.Add(100 * time.Millisecond)
	r.OnTaskComplete("span1", endTime, nil, false)

	if !strings.Contains(stderr.String(), "Completed") {
		t.Errorf("Expected completion message, got: %s", stderr.String())
	}

	if err := r.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestRenderer_PartialLines(t *testing.T) {
	var stdout, stderr bytes.Buffer
	r := linear.NewRenderer(&stdout, &stderr)

	startTime := time.Now()
	r.OnTaskStart("span1", "", "task1", startTime)

	// Send partial line
	r.OnTaskLog("span1", []byte("partial"))
	// Should not be printed yet
	if strings.Contains(stdout.String(), "partial") {
		t.Errorf("Partial line should not be printed immediately")
	}

	// Complete the line
	r.OnTaskLog("span1", []byte(" line\n"))
	if !strings.Contains(stdout.String(), "task1") || !strings.Contains(stdout.String(), "partial line") {
		t.Errorf("Expected complete line, got: %s", stdout.String())
	}

	// Flush on complete
	r.OnTaskLog("span1", []byte("unflushed"))
	endTime := startTime.Add(50 * time.Millisecond)
	r.OnTaskComplete("span1", endTime, nil, false)

	if !strings.Contains(stdout.String(), "task1") || !strings.Contains(stdout.String(), "unflushed") {
		t.Errorf("Expected flushed partial line on complete, got: %s", stdout.String())
	}
}

func TestRenderer_TaskError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	r := linear.NewRenderer(&stdout, &stderr)

	startTime := time.Now()
	r.OnTaskStart("span1", "", "failing-task", startTime)

	r.OnTaskLog("span1", []byte("error output\n"))

	endTime := startTime.Add(50 * time.Millisecond)
	err := zerr.New("task failed")
	r.OnTaskComplete("span1", endTime, err, false)

	stderrStr := stderr.String()
	if !strings.Contains(stderrStr, "Failed") {
		t.Errorf("Expected failure message, got: %s", stderrStr)
	}
	if !strings.Contains(stderrStr, "task failed") {
		t.Errorf("Expected error message, got: %s", stderrStr)
	}
}

func TestRenderer_ConcurrentTasks(t *testing.T) {
	var stdout, stderr bytes.Buffer
	r := linear.NewRenderer(&stdout, &stderr)

	startTime := time.Now()
	r.OnTaskStart("span1", "", "task1", startTime)
	r.OnTaskStart("span2", "", "task2", startTime)

	// Interleaved logs
	r.OnTaskLog("span1", []byte("task1 line 1\n"))
	r.OnTaskLog("span2", []byte("task2 line 1\n"))
	r.OnTaskLog("span1", []byte("task1 line 2\n"))
	r.OnTaskLog("span2", []byte("task2 line 2\n"))

	stdoutStr := stdout.String()
	lines := strings.Split(strings.TrimSpace(stdoutStr), "\n")

	// Verify all lines are prefixed correctly
	expectedPrefixes := map[string]int{
		"[task1]": 2,
		"[task2]": 2,
	}

	for _, line := range lines {
		for prefix := range expectedPrefixes {
			if strings.Contains(line, prefix) {
				expectedPrefixes[prefix]--
			}
		}
	}

	for prefix, count := range expectedPrefixes {
		if count != 0 {
			t.Errorf("Expected prefix %s to appear exactly, remaining: %d", prefix, count)
		}
	}

	endTime := startTime.Add(100 * time.Millisecond)
	r.OnTaskComplete("span1", endTime, nil, false)
	r.OnTaskComplete("span2", endTime, nil, false)
}

func TestRenderer_NoColor(t *testing.T) {
	if err := os.Setenv("NO_COLOR", "1"); err != nil {
		t.Fatalf("Failed to set NO_COLOR: %v", err)
	}
	defer func() {
		_ = os.Unsetenv("NO_COLOR")
	}()

	var stdout, stderr bytes.Buffer
	r := linear.NewRenderer(&stdout, &stderr)

	startTime := time.Now()
	r.OnTaskStart("span1", "", "task1", startTime)

	endTime := startTime.Add(50 * time.Millisecond)
	r.OnTaskComplete("span1", endTime, nil, false)

	// With NO_COLOR, output should not contain ANSI escape codes
	stderrStr := stderr.String()
	if strings.Contains(stderrStr, "\x1b[") {
		t.Errorf("Expected no ANSI codes with NO_COLOR, got: %s", stderrStr)
	}
}

func TestColorAssignment(t *testing.T) {
	tests := []struct {
		name     string
		taskName string
	}{
		{"task1", "task1"},
		{"task2", "task2"},
		{"build", "build"},
		{"test", "test"},
		{"deploy", "deploy"},
	}

	colorSeen := make(map[string]struct{})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			r := linear.NewRenderer(&stdout, &stderr)

			startTime := time.Now()
			r.OnTaskStart("span1", "", tt.taskName, startTime)

			color1 := stderr.String()

			stderr.Reset()
			r.OnTaskStart("span2", "", tt.taskName, startTime.Add(time.Second))

			color2 := stderr.String()

			if color1 != color2 {
				t.Errorf("Same task name %q should produce same color output", tt.taskName)
			}

			if color1 != "" && !strings.Contains(color1, "\x1b[") {
				t.Errorf("Expected ANSI color codes in output for task %q", tt.taskName)
			}

			colorSeen[color1] = struct{}{}
		})
	}

	if len(colorSeen) < 2 {
		t.Errorf("Expected multiple different colors for different tasks, got %d unique colors", len(colorSeen))
	}
}

func TestRenderer_OnTaskLogUnknownSpan(t *testing.T) {
	var stdout, stderr bytes.Buffer
	r := linear.NewRenderer(&stdout, &stderr)

	r.OnTaskLog("unknown-span", []byte("should be ignored\n"))

	if stdout.Len() != 0 {
		t.Errorf("Expected no output for unknown span, got: %s", stdout.String())
	}
}

func TestRenderer_OnTaskCompleteUnknownSpan(t *testing.T) {
	var stdout, stderr bytes.Buffer
	r := linear.NewRenderer(&stdout, &stderr)

	r.OnTaskComplete("unknown-span", time.Now(), nil, false)

	if stderr.Len() != 0 {
		t.Errorf("Expected no output for unknown span completion, got: %s", stderr.String())
	}
}

func TestRenderer_EmptyLines(t *testing.T) {
	var stdout, stderr bytes.Buffer
	r := linear.NewRenderer(&stdout, &stderr)

	startTime := time.Now()
	r.OnTaskStart("span1", "", "task1", startTime)

	r.OnTaskLog("span1", []byte("\n"))
	r.OnTaskLog("span1", []byte("\r\n"))

	if strings.Contains(stdout.String(), "[task1]") {
		t.Errorf("Expected no output for empty lines, got: %s", stdout.String())
	}
}

func TestRenderer_StopFlushesBuffers(t *testing.T) {
	var stdout, stderr bytes.Buffer
	r := linear.NewRenderer(&stdout, &stderr)

	startTime := time.Now()
	r.OnTaskStart("span1", "", "task1", startTime)
	r.OnTaskStart("span2", "", "task2", startTime)

	r.OnTaskLog("span1", []byte("partial1"))
	r.OnTaskLog("span2", []byte("partial2"))

	if err := r.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	stdoutStr := stdout.String()
	if !strings.Contains(stdoutStr, "partial1") {
		t.Errorf("Expected flushed partial1, got: %s", stdoutStr)
	}
	if !strings.Contains(stdoutStr, "partial2") {
		t.Errorf("Expected flushed partial2, got: %s", stdoutStr)
	}
}

func TestRenderer_Wait(t *testing.T) {
	var stdout, stderr bytes.Buffer
	r := linear.NewRenderer(&stdout, &stderr)

	if err := r.Wait(); err != nil {
		t.Errorf("Wait() should not error, got: %v", err)
	}
}

func TestRenderer_NilStdout(_ *testing.T) {
	r := linear.NewRenderer(nil, nil)

	startTime := time.Now()
	r.OnTaskStart("span1", "", "task1", startTime)
	r.OnTaskLog("span1", []byte("test\n"))
	r.OnTaskComplete("span1", startTime.Add(time.Second), nil, false)
}
