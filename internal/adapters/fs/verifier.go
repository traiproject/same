package fs

import (
	"os"
	"path/filepath"

	"go.trai.ch/zerr"
)

// Verifier provides functionality to verify the existence of files.
type Verifier struct{}

// NewVerifier creates a new Verifier.
func NewVerifier() *Verifier {
	return &Verifier{}
}

// VerifyOutputs checks if all output files exist in the given root directory.
// It returns true if all outputs exist, false otherwise.
func (v *Verifier) VerifyOutputs(root string, outputs []string) (bool, error) {
	for _, output := range outputs {
		path := filepath.Join(root, output)
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				return false, nil
			}
			return false, zerr.With(zerr.Wrap(err, "failed to stat output"), "path", path)
		}
	}
	return true, nil
}
