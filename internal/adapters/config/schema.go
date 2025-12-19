package config

// Workfile represents the structure of the bob.work.yaml configuration file.
type Workfile struct {
	Version  string            `yaml:"version"`
	Root     string            `yaml:"root"`
	Tools    map[string]string `yaml:"tools"`
	Projects []string          `yaml:"projects"`
}

// Bobfile represents the structure of the bob.yaml configuration file.
type Bobfile struct {
	Version string              `yaml:"version"`
	Project string              `yaml:"project"`
	Root    string              `yaml:"root"`
	Tools   map[string]string   `yaml:"tools"`
	Tasks   map[string]*TaskDTO `yaml:"tasks"`
}

// TaskDTO represents a task definition in the configuration.
type TaskDTO struct {
	Input       []string          `yaml:"input"`
	Cmd         []string          `yaml:"cmd"`
	Target      []string          `yaml:"target"`
	Tools       []string          `yaml:"tools"`
	DependsOn   []string          `yaml:"dependsOn"`
	Environment map[string]string `yaml:"environment"`
	WorkingDir  string            `yaml:"workingDir"`
}
