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
// Load reads the configuration from the given working directory.
func (l *FileConfigLoader) Load(cwd string) (*domain.Graph, error) {
	path := filepath.Join(cwd, l.Filename)
	return Load(path)
}

type loadedConfig struct {
	ProjectPath string
	Config      Bobfile
}

// Load reads a configuration file from the given path and returns a domain.Graph.
func Load(path string) (*domain.Graph, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, zerr.Wrap(err, "failed to resolve absolute path")
	}

	configs, err := loadRecursively(absPath, make(map[string]bool))
	if err != nil {
		return nil, err
	}

	g := domain.NewGraph()

	// The first config is the root config
	rootConfig := configs[0]
	g.SetRoot(resolveRoot(rootConfig.ProjectPath, rootConfig.Config.Root))

	taskNames := make(map[string]bool)

	// First pass: Collect all task names to verify dependencies later
	for _, lc := range configs {
		for name := range lc.Config.Tasks {
			if taskNames[name] {
				return nil, zerr.With(domain.ErrTaskAlreadyExists, "task_name", name)
			}
			taskNames[name] = true
		}
	}

	// Second pass: Create tasks and add to graph
	for _, lc := range configs {
		if err := createTasks(g, &lc, taskNames); err != nil {
			return nil, err
		}
	}

	return g, nil
}

func createTasks(g *domain.Graph, lc *loadedConfig, taskNames map[string]bool) error {
	for name, dto := range lc.Config.Tasks {
		// Validate reserved task names
		if name == "all" {
			return zerr.With(domain.ErrReservedTaskName, "task_name", name)
		}

		// Validate dependencies exist
		for _, dep := range dto.DependsOn {
			if !taskNames[dep] {
				return zerr.With(domain.ErrMissingDependency, "missing_dependency", dep)
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

		// Set WorkingDir to ProjectPath if not specified
		if dto.WorkingDir == "" {
			task.WorkingDir = domain.NewInternedString(lc.ProjectPath)
		} else {
			if filepath.IsAbs(dto.WorkingDir) {
				task.WorkingDir = domain.NewInternedString(dto.WorkingDir)
			} else {
				task.WorkingDir = domain.NewInternedString(filepath.Join(lc.ProjectPath, dto.WorkingDir))
			}
		}

		if err := g.AddTask(task); err != nil {
			return err
		}
	}
	return nil
}

func loadRecursively(configPath string, visited map[string]bool) ([]loadedConfig, error) {
	if visited[configPath] {
		return nil, nil // Cycle detected or already visited
	}
	visited[configPath] = true

	data, err := os.ReadFile(configPath) //nolint:gosec // path is provided by user
	if err != nil {
		return nil, zerr.Wrap(err, domain.ErrConfigReadFailed.Error())
	}

	var bobfile Bobfile
	if err := yaml.Unmarshal(data, &bobfile); err != nil {
		return nil, zerr.Wrap(err, domain.ErrConfigParseFailed.Error())
	}

	projectPath := filepath.Dir(configPath)
	configs := []loadedConfig{{
		ProjectPath: projectPath,
		Config:      bobfile,
	}}

	for _, pattern := range bobfile.Workspace {
		subConfigs, err := loadWorkspacePattern(projectPath, pattern, visited)
		if err != nil {
			return nil, err
		}
		configs = append(configs, subConfigs...)
	}

	return configs, nil
}

func loadWorkspacePattern(projectPath, pattern string, visited map[string]bool) ([]loadedConfig, error) {
	// Construct absolute glob pattern relative to the current config's directory
	absPattern := pattern
	if !filepath.IsAbs(pattern) {
		absPattern = filepath.Join(projectPath, pattern)
	}

	matches, err := filepath.Glob(absPattern)
	if err != nil {
		return nil, zerr.With(zerr.Wrap(err, "failed to glob workspace pattern"), "pattern", pattern)
	}

	var configs []loadedConfig
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}

		var nextConfigPath string
		switch {
		case info.IsDir():
			nextConfigPath = filepath.Join(match, "bob.yaml")
			if _, statErr := os.Stat(nextConfigPath); statErr != nil {
				continue // No bob.yaml in this directory
			}
		case filepath.Base(match) == "bob.yaml":
			nextConfigPath = match
		default:
			continue // Not a directory or bob.yaml file
		}

		subConfigs, err := loadRecursively(nextConfigPath, visited)
		if err != nil {
			return nil, err
		}
		configs = append(configs, subConfigs...)
	}
	return configs, nil
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

func resolveRoot(configDir, configuredRoot string) string {
	if configuredRoot == "" {
		return filepath.Clean(configDir)
	}
	if filepath.IsAbs(configuredRoot) {
		return filepath.Clean(configuredRoot)
	}
	return filepath.Clean(filepath.Join(configDir, configuredRoot))
}
