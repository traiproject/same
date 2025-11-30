package domain

// Task represents a unit of work in the build system.
// It uses InternedString for fields that are frequently repeated to save memory.
type Task struct {
	Name               InternedString
	Command            []string
	Inputs             []InternedString
	Outputs            []InternedString
	Dependencies       []InternedString
	SystemDependencies []InternedString
	Environment        map[string]string
	WorkingDir         InternedString
}
