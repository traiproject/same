package config

// WorkspaceConfig represents the configuration structure for bob.work.yaml
type WorkspaceConfig struct {
	Version   string   `yaml:"version"`
	Workspace []string `yaml:"workspace"`
}

// Bobfile represents the structure of the bob.yaml configuration file.
type Bobfile struct {
	Version string             `yaml:"version"`
	Project string             `yaml:"project"`
	Root    string             `yaml:"root"`
	Tasks   map[string]TaskDTO `yaml:"tasks"`
}

// TaskDTO represents a task definition in the configuration.
type TaskDTO struct {
	Input       []string          `yaml:"input"`
	Cmd         []string          `yaml:"cmd"`
	Target      []string          `yaml:"target"`
	DependsOn   []string          `yaml:"dependsOn"`
	Environment map[string]string `yaml:"environment"`
	WorkingDir  string            `yaml:"workingDir"`
}
