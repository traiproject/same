// Package config provides the configuration loader for same.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"go.trai.ch/same/internal/core/domain"
	"go.trai.ch/same/internal/core/ports"
	"go.trai.ch/zerr"
	"gopkg.in/yaml.v3"
)

// Loader implements ports.ConfigLoader using a YAML file.
type Loader struct {
	Logger ports.Logger
}

// NewLoader creates a new Loader with the given logger.
func NewLoader(logger ports.Logger) *Loader {
	return &Loader{Logger: logger}
}

// Mode represents the configuration mode of same.
type Mode string

const (
	// ModeWorkspace indicates that same has a workfile.
	ModeWorkspace Mode = "workspace"
	// ModeStandalone indicates that same has only one samefile.
	ModeStandalone Mode = "standalone"
)

var validProjectNameRegex = regexp.MustCompile("^[a-zA-Z0-9_-]+$")

// Load reads a configuration file from the given path and returns a domain.Graph.
func (l *Loader) Load(cwd string) (*domain.Graph, error) {
	configPath, mode, err := l.findConfiguration(cwd)
	if err != nil {
		return nil, err
	}

	switch mode {
	case ModeStandalone:
		return l.loadSamefile(configPath)
	case ModeWorkspace:
		return l.loadWorkfile(configPath)
	default:
		return nil, zerr.With(domain.ErrConfigNotFound, "mode", mode)
	}
}

func (l *Loader) findConfiguration(cwd string) (string, Mode, error) {
	currentDir := cwd
	var standaloneCandidate string

	for {
		workfilePath := filepath.Join(currentDir, domain.WorkFileName)
		if _, err := os.Stat(workfilePath); err == nil {
			return workfilePath, ModeWorkspace, nil
		}

		if standaloneCandidate == "" {
			samefilePath := filepath.Join(currentDir, domain.SameFileName)
			if _, err := os.Stat(samefilePath); err == nil {
				standaloneCandidate = samefilePath
			}
		}

		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			// Reached root
			break
		}
		currentDir = parentDir
	}

	if standaloneCandidate != "" {
		return standaloneCandidate, ModeStandalone, nil
	}

	return "", "", zerr.With(domain.ErrConfigNotFound, "cwd", cwd)
}

func (l *Loader) loadSamefile(configPath string) (*domain.Graph, error) {
	var samefile Samefile
	if err := readAndUnmarshalYAML(configPath, &samefile); err != nil {
		return nil, err
	}

	if samefile.Project != "" {
		l.Logger.Warn(fmt.Sprintf("'project' defined in %s has no effect in standalone mode", domain.SameFileName))
	}

	g := domain.NewGraph()
	g.SetRoot(resolveRoot(configPath, samefile.Root))

	taskNames := make(map[string]bool)

	// First pass: Collect all task names to verify dependencies later
	for name := range samefile.Tasks {
		taskNames[name] = true
	}

	// Second pass: Create tasks and add to graph
	for name := range samefile.Tasks {
		dto := samefile.Tasks[name]
		if err := validateTaskName(name); err != nil {
			return nil, err
		}

		// Validate dependencies exist
		for _, dep := range dto.DependsOn {
			if !taskNames[dep] {
				return nil, zerr.With(domain.ErrMissingDependency, "missing_dependency", dep)
			}
		}

		workingDir := resolveTaskWorkingDir(g.Root(), dto.WorkingDir)

		// Resolve tool aliases to flake references
		taskTools, err := resolveTaskTools(dto.Tools, samefile.Tools)
		if err != nil {
			return nil, zerr.With(err, "task", name)
		}

		task := buildTask(name, dto, workingDir, dto.DependsOn, taskTools)

		if err := g.AddTask(task); err != nil {
			return nil, err
		}
	}

	return g, nil
}

func (l *Loader) loadWorkfile(configPath string) (*domain.Graph, error) {
	var workfile Workfile
	if err := readAndUnmarshalYAML(configPath, &workfile); err != nil {
		return nil, err
	}

	g := domain.NewGraph()
	workspaceRoot := resolveRoot(configPath, workfile.Root)
	g.SetRoot(workspaceRoot)

	projectPaths, err := l.resolveProjectPaths(workspaceRoot, workfile.Projects)
	if err != nil {
		return nil, err
	}

	// Track project names to ensure uniqueness
	projectNames := make(map[string]string)

	// Pass workspace-level tools to all projects
	if err := l.processProjects(g, workspaceRoot, projectPaths, projectNames, workfile.Tools); err != nil {
		return nil, err
	}

	return g, nil
}

func (l *Loader) resolveProjectPaths(workspaceRoot string, patterns []string) ([]string, error) {
	// 1. Resolve Glob Patterns
	// We use a map to deduplicate paths if multiple globs match the same directory
	projectPaths := make(map[string]struct{})

	for _, pattern := range patterns {
		// Join with workspaceRoot to match against absolute paths
		absPattern := filepath.Join(workspaceRoot, pattern)

		matches, err := filepath.Glob(absPattern)
		if err != nil {
			return nil, zerr.Wrap(err, "glob pattern failed: "+pattern)
		}

		for _, match := range matches {
			projectPaths[match] = struct{}{}
		}
	}

	// 2. Sort Paths for Determinism
	// Maps iteration order is random, so we sort the keys to ensure tasks are processed consistently
	sortedPaths := make([]string, 0, len(projectPaths))
	for p := range projectPaths {
		sortedPaths = append(sortedPaths, p)
	}
	slices.Sort(sortedPaths)

	return sortedPaths, nil
}

func (l *Loader) processProjects(
	g *domain.Graph,
	workspaceRoot string,
	projectPaths []string,
	projectNames map[string]string,
	workspaceTools map[string]string,
) error {
	for _, projectPath := range projectPaths {
		if err := l.processProject(g, workspaceRoot, projectPath, projectNames, workspaceTools); err != nil {
			return err
		}
	}
	return nil
}

func (l *Loader) processProject(
	g *domain.Graph,
	workspaceRoot, projectPath string,
	projectNames map[string]string,
	workspaceTools map[string]string,
) error {
	relPath, _ := filepath.Rel(workspaceRoot, projectPath)

	// Check if the match is actually a directory (Glob returns files too)
	info, pathErr := os.Stat(projectPath)
	if pathErr != nil {
		return pathErr
	}
	if !info.IsDir() {
		return nil
	}

	// Check for same.yaml existence
	sameYamlPath := filepath.Join(projectPath, domain.SameFileName)
	if _, fileErr := os.Stat(sameYamlPath); os.IsNotExist(fileErr) {
		l.Logger.Warn(fmt.Sprintf("%s missing in project %s, skipping", domain.SameFileName, relPath))
		return nil
	}

	samefile, err := l.loadSamefileFromPath(sameYamlPath, relPath)
	if err != nil {
		return err
	}

	if err := l.validateSamefile(samefile, relPath); err != nil {
		return err
	}

	// Check for duplicate project names
	if existingPath, exists := projectNames[samefile.Project]; exists {
		err := zerr.With(domain.ErrDuplicateProjectName, "project_name", samefile.Project)
		err = zerr.With(err, "first_occurrence", existingPath)
		err = zerr.With(err, "duplicate_at", relPath)
		return err
	}
	projectNames[samefile.Project] = relPath

	if samefile.Root != "" {
		l.Logger.Warn(fmt.Sprintf("'root' defined in %s is ignored in workspace mode", relPath))
	}

	// Merge tools: workspace tools as base, project tools override
	resolvedTools := mergeTools(workspaceTools, samefile.Tools)

	return l.addProjectTasks(g, samefile, projectPath, resolvedTools)
}

func (l *Loader) loadSamefileFromPath(sameYamlPath, relPath string) (*Samefile, error) {
	// #nosec G304 -- sameYamlPath is constructed from validated projectPath
	projectConfigFile, pathErr := os.ReadFile(sameYamlPath)
	if pathErr != nil {
		pathErr = zerr.Wrap(pathErr, domain.ErrConfigReadFailed.Error())
		pathErr = zerr.With(pathErr, "directory", relPath)
		return nil, pathErr
	}

	var samefile Samefile
	if err := yaml.Unmarshal(projectConfigFile, &samefile); err != nil {
		return nil, zerr.Wrap(err, "failed to parse project config: "+relPath)
	}

	return &samefile, nil
}

func (l *Loader) validateSamefile(samefile *Samefile, relPath string) error {
	if samefile.Project == "" {
		return zerr.With(domain.ErrMissingProjectName, "directory", relPath)
	}

	if !validProjectNameRegex.MatchString(samefile.Project) {
		err := zerr.With(domain.ErrInvalidProjectName, "project_name", samefile.Project)
		return zerr.With(err, "directory", relPath)
	}

	return nil
}

func (l *Loader) addProjectTasks(
	g *domain.Graph,
	samefile *Samefile,
	projectPath string,
	resolvedTools map[string]string,
) error {
	for taskName := range samefile.Tasks {
		dto := samefile.Tasks[taskName]
		if err := validateTaskName(taskName); err != nil {
			return err
		}

		// Rebase inputs and targets to be relative to the workspace root
		var err error
		dto.Input, err = l.rebasePaths(dto.Input, projectPath, g.Root())
		if err != nil {
			return zerr.Wrap(err, "failed to rebase inputs for project "+samefile.Project)
		}

		dto.Target, err = l.rebasePaths(dto.Target, projectPath, g.Root())
		if err != nil {
			return zerr.Wrap(err, "failed to rebase targets for project "+samefile.Project)
		}

		namespacedTaskName := fmt.Sprintf("%s:%s", samefile.Project, taskName)
		namespacedDeps := l.namespaceDependencies(samefile.Project, dto.DependsOn)
		workingDir := resolveTaskWorkingDir(projectPath, dto.WorkingDir)

		// Resolve tool aliases to flake references
		taskTools, err := resolveTaskTools(dto.Tools, resolvedTools)
		if err != nil {
			return zerr.With(err, "task", namespacedTaskName)
		}

		task := buildTask(namespacedTaskName, dto, workingDir, namespacedDeps, taskTools)

		if err := g.AddTask(task); err != nil {
			return err
		}
	}
	return nil
}

func (l *Loader) rebasePaths(paths []string, base, root string) ([]string, error) {
	rebased := make([]string, len(paths))
	for i, p := range paths {
		// Join with base (project path) to get the full path
		abs := filepath.Join(base, p)
		// Make it relative to the workspace root
		rel, err := filepath.Rel(root, abs)
		if err != nil {
			return nil, err
		}
		rebased[i] = rel
	}
	return rebased, nil
}

func (l *Loader) namespaceDependencies(projectName string, deps []string) []string {
	namespacedDeps := make([]string, 0, len(deps))
	for _, dep := range deps {
		if strings.Contains(dep, ":") {
			namespacedDeps = append(namespacedDeps, dep)
		} else {
			namespacedDeps = append(namespacedDeps, fmt.Sprintf("%s:%s", projectName, dep))
		}
	}
	return namespacedDeps
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
	return domain.NewInternedStrings(unique)
}

func resolveRoot(configPath, configuredRoot string) string {
	configDir := filepath.Dir(configPath)
	if configuredRoot == "" {
		return filepath.Clean(configDir)
	}
	if filepath.IsAbs(configuredRoot) {
		return filepath.Clean(configuredRoot)
	}
	return filepath.Clean(filepath.Join(configDir, configuredRoot))
}

// readAndUnmarshalYAML reads a YAML file and unmarshals it into the target struct.
func readAndUnmarshalYAML[T any](configPath string, target *T) error {
	// #nosec G304 -- configPath is validated by caller
	configFile, err := os.ReadFile(configPath)
	if err != nil {
		return zerr.Wrap(err, domain.ErrConfigReadFailed.Error())
	}

	if parseErr := yaml.Unmarshal(configFile, target); parseErr != nil {
		return zerr.Wrap(parseErr, domain.ErrConfigParseFailed.Error())
	}

	return nil
}

// validateTaskName checks if the task name is reserved or contains invalid characters.
func validateTaskName(name string) error {
	if name == "all" {
		return zerr.With(domain.ErrReservedTaskName, "task_name", name)
	}
	if strings.Contains(name, ":") {
		err := zerr.With(domain.ErrInvalidTaskName, "invalid_character", ":")
		return zerr.With(err, "task_name", name)
	}
	return nil
}

// mergeTools creates a new map with workspaceTools as base, project tools overriding.
func mergeTools(workspaceTools, projectTools map[string]string) map[string]string {
	result := make(map[string]string, len(workspaceTools)+len(projectTools))
	for k, v := range workspaceTools {
		result[k] = v
	}
	for k, v := range projectTools {
		result[k] = v
	}
	return result
}

// resolveTaskTools maps tool aliases to their full flake references.
// Returns ErrMissingTool if any alias is not found in resolvedTools.
func resolveTaskTools(aliases []string, resolvedTools map[string]string) (map[string]string, error) {
	if len(aliases) == 0 {
		return nil, nil
	}

	result := make(map[string]string, len(aliases))
	for _, alias := range aliases {
		ref, ok := resolvedTools[alias]
		if !ok {
			return nil, zerr.With(domain.ErrMissingTool, "tool_alias", alias)
		}
		result[alias] = ref
	}
	return result, nil
}

// buildTask creates a domain.Task from a TaskDTO with the given parameters.
func buildTask(
	name string,
	dto *TaskDTO,
	workingDir domain.InternedString,
	deps []string,
	tools map[string]string,
) *domain.Task {
	return &domain.Task{
		Name:         domain.NewInternedString(name),
		Command:      dto.Cmd,
		Inputs:       canonicalizeStrings(dto.Input),
		Outputs:      canonicalizeStrings(dto.Target),
		Dependencies: domain.NewInternedStrings(deps),
		Environment:  dto.Environment,
		WorkingDir:   workingDir,
		Tools:        tools,
	}
}

// resolveTaskWorkingDir resolves the working directory for a task.
// If configuredWorkingDir is empty, uses baseDir.
// If configuredWorkingDir is absolute, uses it directly.
// Otherwise, joins it with baseDir.
func resolveTaskWorkingDir(baseDir, configuredWorkingDir string) domain.InternedString {
	if configuredWorkingDir == "" {
		return domain.NewInternedString(baseDir)
	}

	if filepath.IsAbs(configuredWorkingDir) {
		return domain.NewInternedString(filepath.Clean(configuredWorkingDir))
	}

	return domain.NewInternedString(filepath.Clean(filepath.Join(baseDir, configuredWorkingDir)))
}
