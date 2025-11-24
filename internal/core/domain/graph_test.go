package domain_test

import (
	"testing"

	"go.trai.ch/bob/internal/core/domain"
	"go.trai.ch/zerr"
)

func TestGraph_AddTask(t *testing.T) {
	g := domain.NewGraph()
	task := domain.Task{Name: domain.NewInternedString("task1")}

	if err := g.AddTask(&task); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := g.AddTask(&task); err == nil {
		t.Error("expected error when adding duplicate task, got nil")
	} else {
		// Verify error is of correct type
		if _, ok := err.(*zerr.Error); !ok {
			t.Errorf("expected *zerr.Error, got %T", err)
		}
	}
}

func TestGraph_Validate_Cycle(t *testing.T) {
	g := domain.NewGraph()
	taskA := domain.Task{
		Name:         domain.NewInternedString("A"),
		Dependencies: []domain.InternedString{domain.NewInternedString("B")},
	}
	taskB := domain.Task{
		Name:         domain.NewInternedString("B"),
		Dependencies: []domain.InternedString{domain.NewInternedString("A")},
	}

	if err := g.AddTask(&taskA); err != nil {
		t.Fatalf("failed to add task A: %v", err)
	}
	if err := g.AddTask(&taskB); err != nil {
		t.Fatalf("failed to add task B: %v", err)
	}

	err := g.Validate()
	if err == nil {
		t.Fatal("expected error for cycle, got nil")
	}

	// Verify error is of correct type
	if _, ok := err.(*zerr.Error); !ok {
		t.Fatalf("expected *zerr.Error, got %T", err)
	}
}

func TestGraph_Walk(t *testing.T) {
	g := domain.NewGraph()
	// A -> B -> C
	// Execution order: C, B, A
	taskA := domain.Task{
		Name:         domain.NewInternedString("A"),
		Dependencies: []domain.InternedString{domain.NewInternedString("B")},
	}
	taskB := domain.Task{
		Name:         domain.NewInternedString("B"),
		Dependencies: []domain.InternedString{domain.NewInternedString("C")},
	}
	taskC := domain.Task{
		Name:         domain.NewInternedString("C"),
		Dependencies: []domain.InternedString{},
	}

	if err := g.AddTask(&taskA); err != nil {
		t.Fatalf("failed to add task A: %v", err)
	}
	if err := g.AddTask(&taskB); err != nil {
		t.Fatalf("failed to add task B: %v", err)
	}
	if err := g.AddTask(&taskC); err != nil {
		t.Fatalf("failed to add task C: %v", err)
	}

	if err := g.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	executed := make([]string, 0, 3)
	for task := range g.Walk() {
		executed = append(executed, task.Name.String())
	}

	if len(executed) != 3 {
		t.Fatalf("expected 3 tasks executed, got %d", len(executed))
	}

	if executed[0] != "C" || executed[1] != "B" || executed[2] != "A" {
		t.Errorf("unexpected execution order: %v", executed)
	}
}
