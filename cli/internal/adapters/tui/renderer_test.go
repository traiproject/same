package tui_test

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"go.trai.ch/same/internal/adapters/tui"
	"go.trai.ch/zerr"
)

func TestRenderer_Lifecycle(t *testing.T) {
	model := tui.NewModel(io.Discard)
	renderer := tui.NewRenderer(
		&model,
		tea.WithInput(strings.NewReader("")),
		tea.WithOutput(io.Discard),
		tea.WithoutSignalHandler(),
		tea.WithoutRenderer(),
	)

	ctx := context.Background()
	if err := renderer.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if err := renderer.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	if err := renderer.Wait(); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
}

func TestRenderer_OnPlanEmit(t *testing.T) {
	model := tui.NewModel(io.Discard)
	renderer := tui.NewRenderer(
		&model,
		tea.WithInput(strings.NewReader("")),
		tea.WithOutput(io.Discard),
		tea.WithoutSignalHandler(),
		tea.WithoutRenderer(),
	)

	ctx := context.Background()
	if err := renderer.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() {
		_ = renderer.Stop()
		_ = renderer.Wait()
	}()

	tasks := []string{"task1", "task2"}
	deps := map[string][]string{
		"task2": {"task1"},
	}
	targets := []string{"task2"}

	renderer.OnPlanEmit(tasks, deps, targets)

	time.Sleep(10 * time.Millisecond)
}

func TestRenderer_OnTaskStart(t *testing.T) {
	model := tui.NewModel(io.Discard)
	renderer := tui.NewRenderer(
		&model,
		tea.WithInput(strings.NewReader("")),
		tea.WithOutput(io.Discard),
		tea.WithoutSignalHandler(),
		tea.WithoutRenderer(),
	)

	ctx := context.Background()
	if err := renderer.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() {
		_ = renderer.Stop()
		_ = renderer.Wait()
	}()

	startTime := time.Now()
	renderer.OnTaskStart("span1", "", "task1", startTime)

	time.Sleep(10 * time.Millisecond)
}

func TestRenderer_OnTaskLog(t *testing.T) {
	model := tui.NewModel(io.Discard)
	renderer := tui.NewRenderer(
		&model,
		tea.WithInput(strings.NewReader("")),
		tea.WithOutput(io.Discard),
		tea.WithoutSignalHandler(),
		tea.WithoutRenderer(),
	)

	ctx := context.Background()
	if err := renderer.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() {
		_ = renderer.Stop()
		_ = renderer.Wait()
	}()

	startTime := time.Now()
	renderer.OnTaskStart("span1", "", "task1", startTime)
	renderer.OnTaskLog("span1", []byte("test log line\n"))

	time.Sleep(10 * time.Millisecond)
}

func TestRenderer_OnTaskComplete(t *testing.T) {
	model := tui.NewModel(io.Discard)
	renderer := tui.NewRenderer(
		&model,
		tea.WithInput(strings.NewReader("")),
		tea.WithOutput(io.Discard),
		tea.WithoutSignalHandler(),
		tea.WithoutRenderer(),
	)

	ctx := context.Background()
	if err := renderer.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() {
		_ = renderer.Stop()
		_ = renderer.Wait()
	}()

	startTime := time.Now()
	renderer.OnTaskStart("span1", "", "task1", startTime)
	endTime := startTime.Add(100 * time.Millisecond)
	renderer.OnTaskComplete("span1", endTime, nil)

	time.Sleep(10 * time.Millisecond)
}

func TestRenderer_OnTaskCompleteWithError(t *testing.T) {
	model := tui.NewModel(io.Discard)
	renderer := tui.NewRenderer(
		&model,
		tea.WithInput(strings.NewReader("")),
		tea.WithOutput(io.Discard),
		tea.WithoutSignalHandler(),
		tea.WithoutRenderer(),
	)

	ctx := context.Background()
	if err := renderer.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() {
		_ = renderer.Stop()
		_ = renderer.Wait()
	}()

	startTime := time.Now()
	renderer.OnTaskStart("span1", "", "task1", startTime)
	endTime := startTime.Add(100 * time.Millisecond)
	renderer.OnTaskComplete("span1", endTime, zerr.New("task failed"))

	time.Sleep(10 * time.Millisecond)
}

func TestRenderer_Program(t *testing.T) {
	model := tui.NewModel(io.Discard)
	renderer := tui.NewRenderer(
		&model,
		tea.WithInput(strings.NewReader("")),
		tea.WithOutput(io.Discard),
		tea.WithoutSignalHandler(),
		tea.WithoutRenderer(),
	)

	program := renderer.Program()
	if program == nil {
		t.Error("Expected non-nil Program()")
	}
}
