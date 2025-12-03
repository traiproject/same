// Package config provides the configuration loader for bob.
package config

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

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

	var configs []loadedConfig
	var rootDir string
	var configuredRoot string

	filename := filepath.Base(absPath)
	if filename == "bob.work.yaml" {
		var err error
		configs, configuredRoot, err = loadWorkspace(absPath)
		if err != nil {
			return nil, err
		}
		rootDir = filepath.Dir(absPath)
	} else {
		// Assume standalone project
		lc, err := loadProject(absPath)
		if err != nil {
			return nil, err
		}
		configs = []loadedConfig{lc}
		rootDir = lc.ProjectPath
		configuredRoot = lc.Config.Root
	}

	if err := validateProjectNames(configs); err != nil {
		return nil, err
	}

	mergedTasks := mergeTasks(configs)

	g := domain.NewGraph()
	g.SetRoot(resolveRoot(rootDir, configuredRoot))

	if err := createTasksFromMerged(g, mergedTasks); err != nil {
		return nil, err
	}

	return g, nil
}

// findWorkspaceRoot looks for a workspace root or standalone project.
func findWorkspaceRoot(startDir string) (string, error) {
	absStartDir, err := filepath.Abs(startDir)
	if err != nil {
		return "", zerr.Wrap(err, "failed to resolve absolute path")
	}

	var candidateProject string
	currDir := absStartDir

	for {
		// 1. Check for workspace file
		workspacePath := filepath.Join(currDir, "bob.work.yaml")
		if _, err := os.Stat(workspacePath); err == nil {
			return workspacePath, nil
		}

		// 2. Check for project file
		projectPath := filepath.Join(currDir, "bob.yaml")
		if _, err := os.Stat(projectPath); err == nil {
			// Found a project file, store as candidate if we haven't found one yet
			if candidateProject == "" {
				candidateProject = projectPath
			}
		}

		parentDir := filepath.Dir(currDir)
		if parentDir == currDir {
			// Reached root
			break
		}
		currDir = parentDir
	}

	// If we found a standalone project candidate, return it
	if candidateProject != "" {
		return candidateProject, nil
	}

	return "", zerr.With(domain.ErrConfigReadFailed, "error", "no bob.yaml or bob.work.yaml found")
}

func loadWorkspace(path string) ([]loadedConfig, string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is provided by user
	if err != nil {
		return nil, "", zerr.Wrap(err, domain.ErrConfigReadFailed.Error())
	}

	var workspace BobWorkspace
	if err := yaml.Unmarshal(data, &workspace); err != nil {
		return nil, "", zerr.Wrap(err, domain.ErrConfigParseFailed.Error())
	}

	workspaceDir := filepath.Dir(path)
	var configs []loadedConfig

	for _, pattern := range workspace.Workspace {
		// Construct absolute glob pattern relative to the workspace directory
		absPattern := pattern
		if !filepath.IsAbs(pattern) {
			absPattern = filepath.Join(workspaceDir, pattern)
		}

		matches, err := filepath.Glob(absPattern)
		if err != nil {
			return nil, "", zerr.With(zerr.Wrap(err, "failed to glob workspace pattern"), "pattern", pattern)
		}

		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil {
				continue
			}

			var projectPath string
			switch {
			case info.IsDir():
				projectPath = filepath.Join(match, "bob.yaml")
			case filepath.Base(match) == "bob.yaml":
				projectPath = match
			default:
				continue
			}

			// Check if file exists
			if _, err := os.Stat(projectPath); err != nil {
				continue
			}

			lc, err := loadProject(projectPath)
			if err != nil {
				return nil, "", err
			}
			configs = append(configs, lc)
		}
	}

	return configs, workspace.Root, nil
}

func loadProject(path string) (loadedConfig, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is provided by user
	if err != nil {
		return loadedConfig{}, zerr.Wrap(err, domain.ErrConfigReadFailed.Error())
	}

	var bobfile Bobfile
	if err := yaml.Unmarshal(data, &bobfile); err != nil {
		return loadedConfig{}, zerr.Wrap(err, domain.ErrConfigParseFailed.Error())
	}

	// Validation: Ensure no workspace field is present?
	// The struct doesn't have it, so yaml.Unmarshal will ignore it.
	// We could parse into map[string]interface{} to strictly forbid it,
	// but the instructions say "triggers a parsing error or is strictly ignored".
	// Since I removed the field from the struct, it is strictly ignored.
	// For stricter validation, we can check raw yaml, but "strictly ignored" is acceptable.

	return loadedConfig{
		ProjectPath: filepath.Dir(path),
		Config:      bobfile,
	}, nil
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
