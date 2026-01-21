package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/same/internal/core/domain"
)

func TestGraph_Cycle(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*domain.Graph)
		wantErr     bool
		errContains string
	}{
		{
			name: "Simple Cycle A->A",
			setup: func(g *domain.Graph) {
				tA := &domain.Task{
					Name:         domain.NewInternedString("A"),
					Dependencies: []domain.InternedString{domain.NewInternedString("A")},
				}
				_ = g.AddTask(tA)
			},
			wantErr:     true,
			errContains: "cycle detected",
		},
		{
			name: "Two Node Cycle A->B->A",
			setup: func(g *domain.Graph) {
				tA := &domain.Task{
					Name:         domain.NewInternedString("A"),
					Dependencies: []domain.InternedString{domain.NewInternedString("B")},
				}
				tB := &domain.Task{
					Name:         domain.NewInternedString("B"),
					Dependencies: []domain.InternedString{domain.NewInternedString("A")},
				}
				_ = g.AddTask(tA)
				_ = g.AddTask(tB)
			},
			wantErr:     true,
			errContains: "cycle detected",
		},
		{
			name: "Three Node Cycle A->B->C->A",
			setup: func(g *domain.Graph) {
				tA := &domain.Task{
					Name:         domain.NewInternedString("A"),
					Dependencies: []domain.InternedString{domain.NewInternedString("B")},
				}
				tB := &domain.Task{
					Name:         domain.NewInternedString("B"),
					Dependencies: []domain.InternedString{domain.NewInternedString("C")},
				}
				tC := &domain.Task{
					Name:         domain.NewInternedString("C"),
					Dependencies: []domain.InternedString{domain.NewInternedString("A")},
				}
				_ = g.AddTask(tA)
				_ = g.AddTask(tB)
				_ = g.AddTask(tC)
			},
			wantErr:     true,
			errContains: "cycle detected",
		},
		{
			name: "No Cycle A->B->C",
			setup: func(g *domain.Graph) {
				tA := &domain.Task{
					Name:         domain.NewInternedString("A"),
					Dependencies: []domain.InternedString{domain.NewInternedString("B")},
				}
				tB := &domain.Task{
					Name:         domain.NewInternedString("B"),
					Dependencies: []domain.InternedString{domain.NewInternedString("C")},
				}
				tC := &domain.Task{
					Name: domain.NewInternedString("C"),
				}
				_ = g.AddTask(tA)
				_ = g.AddTask(tB)
				_ = g.AddTask(tC)
			},
			wantErr: false,
		},
		{
			name: "Disconnected Components No Cycle",
			setup: func(g *domain.Graph) {
				// A->B
				tA := &domain.Task{
					Name:         domain.NewInternedString("A"),
					Dependencies: []domain.InternedString{domain.NewInternedString("B")},
				}
				tB := &domain.Task{
					Name: domain.NewInternedString("B"),
				}
				// C->D
				tC := &domain.Task{
					Name:         domain.NewInternedString("C"),
					Dependencies: []domain.InternedString{domain.NewInternedString("D")},
				}
				tD := &domain.Task{
					Name: domain.NewInternedString("D"),
				}
				_ = g.AddTask(tA)
				_ = g.AddTask(tB)
				_ = g.AddTask(tC)
				_ = g.AddTask(tD)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := domain.NewGraph()
			tt.setup(g)
			err := g.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGraph_TopologicalSort(t *testing.T) {
	// A -> B, C
	// B -> D
	// C -> D
	// D -> (leaf)
	// Order should be roughly D, B, C, A or D, C, B, A (depending on valid topological sort)
	// Since Validate sorts alphabetically for disconnected components or when neighbors are equal,
	// we expect specific order if deterministic.

	g := domain.NewGraph()
	tA := &domain.Task{
		Name:         domain.NewInternedString("A"),
		Dependencies: []domain.InternedString{domain.NewInternedString("B"), domain.NewInternedString("C")},
	}
	tB := &domain.Task{
		Name:         domain.NewInternedString("B"),
		Dependencies: []domain.InternedString{domain.NewInternedString("D")},
	}
	tC := &domain.Task{
		Name:         domain.NewInternedString("C"),
		Dependencies: []domain.InternedString{domain.NewInternedString("D")},
	}
	tD := &domain.Task{
		Name: domain.NewInternedString("D"),
	}

	require.NoError(t, g.AddTask(tA))
	require.NoError(t, g.AddTask(tB))
	require.NoError(t, g.AddTask(tC))
	require.NoError(t, g.AddTask(tD))

	require.NoError(t, g.Validate())

	var execOrder []string //nolint:prealloc // Graph size is not easily accessible here
	for task := range g.Walk() {
		execOrder = append(execOrder, task.Name.String())
	}

	// Validate dependencies are met
	seen := make(map[string]bool)
	for _, taskName := range execOrder {
		task, found := g.GetTask(domain.NewInternedString(taskName))
		require.True(t, found)
		for _, dep := range task.Dependencies {
			assert.True(t, seen[dep.String()], "Dependency %s must be executed before %s", dep, taskName)
		}
		seen[taskName] = true
	}

	// For A->B,C; B->D; C->D
	// Dependencies:
	// A needs BO C
	// B needs D
	// C needs D
	// D needs nothing
	// D must range first.
	// B, C must be after D.
	// A must be after B, C.
	assert.Equal(t, "D", execOrder[0])
	assert.Equal(t, "A", execOrder[3])
	assert.Contains(t, execOrder[1:3], "B")
	assert.Contains(t, execOrder[1:3], "C")
}

func TestGenerateEnvID(t *testing.T) {
	t.Run("Deterministic", func(t *testing.T) {
		tools1 := map[string]string{
			"go":   "1.21",
			"node": "20",
			"rust": "1.75",
		}
		tools2 := map[string]string{
			"rust": "1.75",
			"go":   "1.21",
			"node": "20",
		}

		hash1 := domain.GenerateEnvID(tools1)
		hash2 := domain.GenerateEnvID(tools2)

		assert.NotEmpty(t, hash1)
		assert.Equal(t, hash1, hash2, "Hash should be deterministic dependent of map iteration order")
	})

	t.Run("Changes on content", func(t *testing.T) {
		tools1 := map[string]string{
			"go": "1.21",
		}
		tools2 := map[string]string{
			"go": "1.22",
		}

		hash1 := domain.GenerateEnvID(tools1)
		hash2 := domain.GenerateEnvID(tools2)

		assert.NotEqual(t, hash1, hash2, "Hash should change if content changes")
	})

	t.Run("Empty", func(t *testing.T) {
		tools := map[string]string{}
		hash := domain.GenerateEnvID(tools)
		assert.NotEmpty(t, hash)

		// Hardcoded check to ensure golden value stability if needed (SHA-256 of empty string)
		// sha256("") = e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
		assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", hash)
	})
}
