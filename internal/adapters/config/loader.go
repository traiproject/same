// Package config provides the configuration loader for bob.
package config

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"go.trai.ch/bob/internal/core/domain"
	"go.trai.ch/bob/internal/core/ports"
	"go.trai.ch/zerr"
	"gopkg.in/yaml.v3"
)

// FileConfigLoader implements ports.ConfigLoader using a YAML file.
type FileConfigLoader struct {
	Filename string
	logger   ports.Logger
}

// Load reads the configuration from the given working directory.
// Load reads the configuration from the given working directory.
func (l *FileConfigLoader) Load(cwd string) (*domain.Graph, error) {
	// Find the workspace root or fallback to the nearest config
	rootPath, err := findWorkspaceRoot(cwd)
	if err != nil {
		return nil, err
	}
	return Load(rootPath)
}

type loadedConfig struct {
	ProjectPath string
	Config      Bobfile
}

type mergedTask struct {
	Task    TaskDTO
	Project string
	Path    string
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

	if err := validateProjectNames(configs); err != nil {
		return nil, err
	}

	mergedTasks := mergeTasks(configs)

	g := domain.NewGraph()
	g.SetRoot(resolveRoot(configs[0].ProjectPath, configs[0].Config.Root))

	if err := createTasksFromMerged(g, mergedTasks); err != nil {
		return nil, err
	}

	return g, nil
}

// findWorkspaceRoot looks for a bob.yaml file starting from startDir and traversing upwards.
// It returns the path to the config file that should be used as the root.
func findWorkspaceRoot(startDir string) (string, error) {
	absStartDir, err := filepath.Abs(startDir)
	if err != nil {
		return "", zerr.Wrap(err, "failed to resolve absolute path")
	}

	var nearestConfig string
	currDir := absStartDir

	for {
		configPath := filepath.Join(currDir, "bob.yaml")
		if _, err := os.Stat(configPath); err == nil {
			// Found a config file
			if nearestConfig == "" {
				nearestConfig = configPath
			}

			// Check if it's a workspace root
			// If we can't read/parse it, we treat it as not a workspace and continue searching.
			if isWorkspace, err := isWorkspaceConfig(configPath); err == nil && isWorkspace {
				return configPath, nil
			}
		}

		parentDir := filepath.Dir(currDir)
		if parentDir == currDir {
			// Reached root
			break
		}
		currDir = parentDir
	}

	if nearestConfig != "" {
		return nearestConfig, nil
	}

	// Fallback to startDir/bob.yaml
	return filepath.Join(absStartDir, "bob.yaml"), nil
}

func isWorkspaceConfig(path string) (bool, error) {
	projectPath := filepath.Dir(path)
	workspaceConfigPath := filepath.Join(projectPath, "bob.work.yaml")

	if _, err := os.Stat(workspaceConfigPath); err == nil {
		// Workspace config file exists
		return true, nil
	}

	// No workspace config file found
	return false, nil
}

func validateProjectNames(configs []loadedConfig) error {
	projectNames := make(map[string]bool)
	validName := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

	for _, lc := range configs {
		if lc.Config.Project == "" {
			err := zerr.With(domain.ErrInvalidConfig, "error", "missing project name")
			return zerr.With(err, "file", lc.ProjectPath)
		}
		if lc.Config.Project == "all" {
			err := zerr.With(domain.ErrReservedTaskName, "project", lc.Config.Project)
			return zerr.With(err, "file", lc.ProjectPath)
		}
		if !validName.MatchString(lc.Config.Project) {
			err := zerr.With(
				domain.ErrInvalidConfig,
				"error", "project name must only contain alphanumeric characters, underscores or hyphens",
			)
			return zerr.With(err, "project", lc.Config.Project)
		}
		if projectNames[lc.Config.Project] {
			err := zerr.With(domain.ErrInvalidConfig, "error", "duplicate project name")
			return zerr.With(err, "project", lc.Config.Project)
		}
		projectNames[lc.Config.Project] = true
	}
	return nil
}

func mergeTasks(configs []loadedConfig) map[string]mergedTask {
	mergedTasks := make(map[string]mergedTask)
	for _, lc := range configs {
		for name, task := range lc.Config.Tasks {
			fullName := lc.Config.Project + ":" + name
			mergedTasks[fullName] = mergedTask{
				Task:    task,
				Project: lc.Config.Project,
				Path:    lc.ProjectPath,
			}
		}
	}
	return mergedTasks
}

func createTasksFromMerged(g *domain.Graph, mergedTasks map[string]mergedTask) error {
	for fullName := range mergedTasks {
		info := mergedTasks[fullName]
		// Extract task name from fullName (after ":")
		parts := strings.SplitN(fullName, ":", 2)
		taskName := parts[1]

		if err := validateReservedTaskName(taskName, info.Project); err != nil {
			return err
		}

		rewrittenDeps, err := rewriteDependencies(&info, mergedTasks)
		if err != nil {
			return zerr.With(err, "task", fullName)
		}

		domainTask := buildDomainTask(fullName, &info, rewrittenDeps)
		if err := g.AddTask(domainTask); err != nil {
			return err
		}
	}
	return nil
}

func validateReservedTaskName(taskName, project string) error {
	if taskName == "all" {
		err := zerr.With(domain.ErrReservedTaskName, "task_name", taskName)
		return zerr.With(err, "project", project)
	}
	return nil
}

func rewriteDependencies(info *mergedTask, mergedTasks map[string]mergedTask) ([]string, error) {
	rewrittenDeps := make([]string, len(info.Task.DependsOn))
	for i, dep := range info.Task.DependsOn {
		resolvedDep := dep
		if !strings.Contains(dep, ":") {
			resolvedDep = info.Project + ":" + dep
		}

		if _, exists := mergedTasks[resolvedDep]; !exists {
			return nil, zerr.With(domain.ErrMissingDependency, "missing_dependency", resolvedDep)
		}
		rewrittenDeps[i] = resolvedDep
	}
	return rewrittenDeps, nil
}

func buildDomainTask(fullName string, info *mergedTask, rewrittenDeps []string) *domain.Task {
	task := &domain.Task{
		Name:         domain.NewInternedString(fullName),
		Command:      info.Task.Cmd,
		Inputs:       canonicalizeStrings(info.Task.Input),
		Outputs:      canonicalizeStrings(info.Task.Target),
		Dependencies: internStrings(rewrittenDeps),
		Environment:  info.Task.Environment,
	}

	switch {
	case info.Task.WorkingDir == "":
		task.WorkingDir = domain.NewInternedString(info.Path)
	case filepath.IsAbs(info.Task.WorkingDir):
		task.WorkingDir = domain.NewInternedString(info.Task.WorkingDir)
	default:
		task.WorkingDir = domain.NewInternedString(filepath.Join(info.Path, info.Task.WorkingDir))
	}

	return task
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

	// Check for workspace configuration file
	workspaceConfigPath := filepath.Join(projectPath, "bob.work.yaml")
	if workspaceData, err := os.ReadFile(workspaceConfigPath); err == nil {
		var workspaceConfig WorkspaceConfig
		if err := yaml.Unmarshal(workspaceData, &workspaceConfig); err == nil {
			for _, pattern := range workspaceConfig.Workspace {
				subConfigs, err := loadWorkspacePattern(projectPath, pattern, visited)
				if err != nil {
					return nil, err
				}
				configs = append(configs, subConfigs...)
			}
		}
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

// NewFileConfigLoader creates a new FileConfigLoader with the given filesystem and logger.
func NewFileConfigLoader(filename string, logger ports.Logger) *FileConfigLoader {
	return &FileConfigLoader{
		Filename: filename,
		logger:   logger,
	}
}
