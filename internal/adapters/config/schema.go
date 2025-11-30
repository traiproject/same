package config

// Bobfile represents the structure of the bob.yaml configuration file.
type Bobfile struct {
	Version string              `yaml:"version"`
	Root    string              `yaml:"root"`
	Tasks   map[string]*TaskDTO `yaml:"tasks"`
}

// TaskDTO represents a task definition in the configuration.
type TaskDTO struct {
	Input        []string          `yaml:"input"`
	Cmd          []string          `yaml:"cmd"`
	Target       []string          `yaml:"target"`
	DependsOn    []string          `yaml:"dependsOn"`
	Dependencies []string          `yaml:"dependencies"`
	Environment  map[string]string `yaml:"environment"`
	WorkingDir   string            `yaml:"workingDir"`
}
