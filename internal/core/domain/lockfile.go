package domain

// Lockfile represents the complete state of resolved package dependencies.
// It provides a reproducible snapshot of all dependencies across architectures.
type Lockfile struct {
	// Version is the lockfile format version (e.g., 1, 2).
	// This allows for future schema migrations and backward compatibility.
	Version int

	// Packages maps canonical package names to their resolved package information.
	// The key is the package name as a string for serialization compatibility.
	Packages map[string]ResolvedPackage
}
