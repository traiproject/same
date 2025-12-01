package domain

// DependencyRequest represents a user's intent to declare a system dependency.
// This is the input representation before resolution (e.g., from bob.yaml).
type DependencyRequest struct {
	// Name is the package name as requested by the user (e.g., "go", "nodejs").
	Name InternedString

	// Version is the requested version constraint (e.g., "1.24.0", "latest").
	Version InternedString
}
