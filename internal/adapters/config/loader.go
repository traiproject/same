// Package config provides the configuration loader for bob.
package config

import (
	"fmt"
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

// Loader implements ports.ConfigLoader using a YAML file.
type Loader struct {
	Logger ports.Logger
}

// NewLoader creates a new Loader with the given logger.
func NewLoader(logger ports.Logger) *Loader {
	return &Loader{Logger: logger}
}

// Mode represents the configuration mode of bob.
type Mode string

const (
	// WorkfileName represents the name of a workfile.
	WorkfileName = "bob.work.yaml"
	// BobfileName represents the name of a bobfile.
	BobfileName = "bob.yaml"
	// ModeWorkspace indicates that bob has a workfile.
	ModeWorkspace Mode = "workspace"
	// ModeStandalone indicates that bob has only one bobfile.
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
		return l.loadBobfile(configPath)
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
		workfilePath := filepath.Join(currentDir, WorkfileName)
		if _, err := os.Stat(workfilePath); err == nil {
			return workfilePath, ModeWorkspace, nil
		}

		if standaloneCandidate == "" {
			bobfilePath := filepath.Join(currentDir, BobfileName)
			if _, err := os.Stat(bobfilePath); err == nil {
				standaloneCandidate = bobfilePath
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

func (l *Loader) loadBobfile(configPath string) (*domain.Graph, error) {
	var bobfile Bobfile
	if err := readAndUnmarshalYAML(configPath, &bobfile); err != nil {
		return nil, err
	}

	if bobfile.Project != "" {
		l.Logger.Warn(fmt.Sprintf("'project' defined in %s has no effect in standalone mode", BobfileName))
	}

	g := domain.NewGraph()
	g.SetRoot(resolveRoot(configPath, bobfile.Root))

	taskNames := make(map[string]bool)

	// First pass: Collect all task names to verify dependencies later
	for name := range bobfile.Tasks {
		taskNames[name] = true
	}

	// Second pass: Create tasks and add to graph
	for name := range bobfile.Tasks {
		dto := bobfile.Tasks[name]
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
		task := buildTask(name, dto, workingDir, dto.DependsOn)

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

	if err := l.processProjects(g, workspaceRoot, projectPaths, projectNames); err != nil {
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
	g *domain.Graph, workspaceRoot string, projectPaths []string, projectNames map[string]string,
) error {
	for _, projectPath := range projectPaths {
		if err := l.processProject(g, workspaceRoot, projectPath, projectNames); err != nil {
			return err
		}
	}
	return nil
}

func (l *Loader) processProject(
	g *domain.Graph, workspaceRoot, projectPath string, projectNames map[string]string,
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

	// Check for bob.yaml existence
	bobYamlPath := filepath.Join(projectPath, BobfileName)
	if _, fileErr := os.Stat(bobYamlPath); os.IsNotExist(fileErr) {
		l.Logger.Warn(fmt.Sprintf("%s missing in project %s, skipping", BobfileName, relPath))
		return nil
	}

	bobfile, err := l.loadBobfileFromPath(bobYamlPath, relPath)
	if err != nil {
		return err
	}

	if err := l.validateBobfile(bobfile, relPath); err != nil {
		return err
	}

	// Check for duplicate project names
	if existingPath, exists := projectNames[bobfile.Project]; exists {
		err := zerr.With(domain.ErrDuplicateProjectName, "project_name", bobfile.Project)
		err = zerr.With(err, "first_occurrence", existingPath)
		err = zerr.With(err, "duplicate_at", relPath)
		return err
	}
	projectNames[bobfile.Project] = relPath

	if bobfile.Root != "" {
		l.Logger.Warn(fmt.Sprintf("'root' defined in %s is ignored in workspace mode", relPath))
	}

	return l.addProjectTasks(g, bobfile, projectPath)
}

func (l *Loader) loadBobfileFromPath(bobYamlPath, relPath string) (*Bobfile, error) {
	// #nosec G304 -- bobYamlPath is constructed from validated projectPath
	projectConfigFile, pathErr := os.ReadFile(bobYamlPath)
	if pathErr != nil {
		pathErr = zerr.Wrap(pathErr, domain.ErrConfigReadFailed.Error())
		pathErr = zerr.With(pathErr, "directory", relPath)
		return nil, pathErr
	}

	var bobfile Bobfile
	if err := yaml.Unmarshal(projectConfigFile, &bobfile); err != nil {
		return nil, zerr.Wrap(err, "failed to parse project config: "+relPath)
	}

	return &bobfile, nil
}

func (l *Loader) validateBobfile(bobfile *Bobfile, relPath string) error {
	if bobfile.Project == "" {
		return zerr.With(domain.ErrMissingProjectName, "directory", relPath)
	}

	if !validProjectNameRegex.MatchString(bobfile.Project) {
		err := zerr.With(domain.ErrInvalidProjectName, "project_name", bobfile.Project)
		return zerr.With(err, "directory", relPath)
	}

	return nil
}

func (l *Loader) addProjectTasks(g *domain.Graph, bobfile *Bobfile, projectPath string) error {
	for taskName := range bobfile.Tasks {
		dto := bobfile.Tasks[taskName]
		if err := validateTaskName(taskName); err != nil {
			return err
		}

		// Rebase inputs and targets to be relative to the workspace root
		var err error
		dto.Input, err = l.rebasePaths(dto.Input, projectPath, g.Root())
		if err != nil {
			return zerr.Wrap(err, "failed to rebase inputs for project "+bobfile.Project)
		}

		dto.Target, err = l.rebasePaths(dto.Target, projectPath, g.Root())
		if err != nil {
			return zerr.Wrap(err, "failed to rebase targets for project "+bobfile.Project)
		}

		namespacedTaskName := fmt.Sprintf("%s:%s", bobfile.Project, taskName)
		namespacedDeps := l.namespaceDependencies(bobfile.Project, dto.DependsOn)
		workingDir := resolveTaskWorkingDir(projectPath, dto.WorkingDir)

		task := buildTask(namespacedTaskName, dto, workingDir, namespacedDeps)

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

// buildTask creates a domain.Task from a TaskDTO with the given parameters.
func buildTask(name string, dto *TaskDTO, workingDir domain.InternedString, deps []string) *domain.Task {
	return &domain.Task{
		Name:         domain.NewInternedString(name),
		Command:      dto.Cmd,
		Inputs:       canonicalizeStrings(dto.Input),
		Outputs:      canonicalizeStrings(dto.Target),
		Dependencies: domain.NewInternedStrings(deps),
		Environment:  dto.Environment,
		WorkingDir:   workingDir,
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
