// Package config provides the configuration loader for bob.
package config

import (
	"os"

	"gopkg.in/yaml.v3"

	"go.trai.ch/bob/internal/core/domain"
	"go.trai.ch/zerr"
)

// Bobfile represents the structure of the bob.yaml configuration file.
type Bobfile struct {
	Version string             `yaml:"version"`
	Tasks   map[string]TaskDTO `yaml:"tasks"`
}

// TaskDTO represents a task definition in the configuration.
type TaskDTO struct {
	Input     []string `yaml:"input"`
	Cmd       []string `yaml:"cmd"`
	Target    []string `yaml:"target"`
	DependsOn []string `yaml:"dependsOn"`
}

// Load reads a configuration file from the given path and returns a domain.Graph.
func Load(path string) (*domain.Graph, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is provided by user
	if err != nil {
		return nil, zerr.Wrap(err, "failed to read config file")
	}

	var bobfile Bobfile
	if err := yaml.Unmarshal(data, &bobfile); err != nil {
		return nil, zerr.Wrap(err, "failed to parse config file")
	}

	g := domain.NewGraph()
	taskNames := make(map[string]bool)

	// First pass: Collect all task names to verify dependencies later
	for name := range bobfile.Tasks {
		taskNames[name] = true
	}

	// Second pass: Create tasks and add to graph
	for name, dto := range bobfile.Tasks {
		// Validate dependencies exist
		for _, dep := range dto.DependsOn {
			if !taskNames[dep] {
				return nil, zerr.With(zerr.New("missing dependency"), "missing_dependency", dep)
			}
		}

		task := &domain.Task{
			Name:         domain.NewInternedString(name),
			Command:      dto.Cmd,
			Inputs:       internStrings(dto.Input),
			Outputs:      internStrings(dto.Target),
			Dependencies: internStrings(dto.DependsOn),
		}

		if err := g.AddTask(task); err != nil {
			return nil, err
		}
	}

	return g, nil
}

func internStrings(strs []string) []domain.InternedString {
	res := make([]domain.InternedString, len(strs))
	for i, s := range strs {
		res[i] = domain.NewInternedString(s)
	}
	return res
}
