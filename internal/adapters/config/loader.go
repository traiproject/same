// Package config provides the configuration loader for bob.
package config

import (
	"os"
	"path/filepath"
	"slices"

	"go.trai.ch/bob/internal/core/domain"
	"go.trai.ch/zerr"
	"gopkg.in/yaml.v3"
)

// FileConfigLoader implements ports.ConfigLoader using a YAML file.
type FileConfigLoader struct {
	Filename string
}

// Load reads the configuration from the given working directory.
func (l *FileConfigLoader) Load(cwd string) (*domain.Graph, error) {
	path := filepath.Join(cwd, l.Filename)
	return Load(path)
}

// Bobfile represents the structure of the bob.yaml configuration file.
type Bobfile struct {
	Version string             `yaml:"version"`
	Tasks   map[string]TaskDTO `yaml:"tasks"`
}

// TaskDTO represents a task definition in the configuration.
type TaskDTO struct {
	Input       []string          `yaml:"input"`
	Cmd         []string          `yaml:"cmd"`
	Target      []string          `yaml:"target"`
	DependsOn   []string          `yaml:"dependsOn"`
	Environment map[string]string `yaml:"environment"`
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
		// Validate reserved task names
		if name == "all" {
			return nil, zerr.With(zerr.New("task name 'all' is reserved"), "task_name", name)
		}

		// Validate dependencies exist
		for _, dep := range dto.DependsOn {
			if !taskNames[dep] {
				return nil, zerr.With(zerr.New("missing dependency"), "missing_dependency", dep)
			}
		}

		task := &domain.Task{
			Name:         domain.NewInternedString(name),
			Command:      dto.Cmd,
			Inputs:       canonicalizeStrings(dto.Input),
			Outputs:      canonicalizeStrings(dto.Target),
			Dependencies: internStrings(dto.DependsOn),
			Environment:  dto.Environment,
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

func canonicalizeStrings(strs []string) []domain.InternedString {
	if len(strs) == 0 {
		return nil
	}

	// Sort strings
	sorted := make([]string, len(strs))
	copy(sorted, strs)
	slices.Sort(sorted)

	// Deduplicate and intern
	unique := slices.Compact(sorted)
	res := make([]domain.InternedString, len(unique))
	for i, s := range unique {
		res[i] = domain.NewInternedString(s)
	}
	return res
}
