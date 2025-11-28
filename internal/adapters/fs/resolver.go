package fs

import (
	"path/filepath"
	"sort"

	"go.trai.ch/bob/internal/core/ports"
	"go.trai.ch/zerr"
)

var _ ports.InputResolver = (*Resolver)(nil)

// Resolver implements the InputResolver interface using filepath.Glob.
type Resolver struct{}

// NewResolver creates a new Resolver.
func NewResolver() *Resolver {
	return &Resolver{}
}

// ResolveInputs resolves the given input patterns to a list of concrete file paths.
func (r *Resolver) ResolveInputs(inputs []string, root string) ([]string, error) {
	uniquePaths := make(map[string]bool)

	for _, input := range inputs {
		path := filepath.Join(root, input)

		// Check if it's a glob pattern
		matches, err := filepath.Glob(path)
		if err != nil {
			return nil, zerr.With(zerr.Wrap(err, "failed to glob path"), "path", path)
		}

		if len(matches) == 0 {
			// If no matches are found, we return "input not found" to indicate that
			// the input pattern did not resolve to any files. This behavior is
			// consistent with the original implementation and ensures users are
			// notified when their input does not match any files. For both direct
			// file paths and glob patterns, zero matches are treated as
			// "input not found" to provide clear feedback.
			//
			// Rationale: This approach avoids silent failures and helps users identify issues with their input patterns.
			return nil, zerr.With(zerr.New("input not found"), "path", path)
		}

		for _, match := range matches {
			uniquePaths[match] = true
		}
	}

	// Convert map to slice and sort
	result := make([]string, 0, len(uniquePaths))
	for path := range uniquePaths {
		result = append(result, path)
	}
	sort.Strings(result)

	return result, nil
}
