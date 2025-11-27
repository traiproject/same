package fs

import (
	"encoding/binary"
	"fmt"
	"io"
	iofs "io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/cespare/xxhash/v2"
	"go.trai.ch/bob/internal/core/domain"
	"go.trai.ch/bob/internal/core/ports"
	"go.trai.ch/zerr"
)

var _ ports.Hasher = (*Hasher)(nil)

// Hasher provides hashing functionality for tasks and files.
type Hasher struct {
	walker *Walker
}

// NewHasher creates a new Hasher.
func NewHasher(walker *Walker) *Hasher {
	return &Hasher{walker: walker}
}

// ComputeFileHash computes the XXHash of a file's content.
func (h *Hasher) ComputeFileHash(path string) (uint64, error) {
	f, err := os.Open(path) //nolint:gosec // Path is controlled by caller
	if err != nil {
		return 0, zerr.With(zerr.Wrap(err, "failed to open file"), "path", path)
	}
	defer f.Close() //nolint:errcheck // Best effort close in defer

	hasher := xxhash.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return 0, zerr.With(zerr.Wrap(err, "failed to hash file content"), "path", path)
	}

	return hasher.Sum64(), nil
}

// ComputeInputHash computes a single hash representing the task configuration,
// environment, and input files.
func (h *Hasher) ComputeInputHash(task *domain.Task, env map[string]string, root string) (string, error) {
	hasher := xxhash.New()

	h.hashTaskDefinition(task, hasher)
	h.hashEnvironment(env, hasher)

	if err := h.hashInputFiles(task, root, hasher); err != nil {
		return "", err
	}

	return fmt.Sprintf("%016x", hasher.Sum64()), nil
}

// hashTaskDefinition hashes the task's name, inputs, outputs, and dependencies.
func (h *Hasher) hashTaskDefinition(task *domain.Task, hasher *xxhash.Digest) {
	// Name
	_, _ = hasher.WriteString(task.Name.String())
	_, _ = hasher.Write([]byte{0}) // Separator

	// Inputs
	for _, input := range task.Inputs {
		_, _ = hasher.WriteString(input.String())
		_, _ = hasher.Write([]byte{0})
	}
	_, _ = hasher.Write([]byte{0}) // Section separator

	// Outputs
	for _, output := range task.Outputs {
		_, _ = hasher.WriteString(output.String())
		_, _ = hasher.Write([]byte{0})
	}
	_, _ = hasher.Write([]byte{0})

	// Dependencies
	for _, dep := range task.Dependencies {
		_, _ = hasher.WriteString(dep.String())
		_, _ = hasher.Write([]byte{0})
	}
	_, _ = hasher.Write([]byte{0})
}

// hashEnvironment hashes environment variables in a deterministic order.
func (h *Hasher) hashEnvironment(env map[string]string, hasher *xxhash.Digest) {
	// Sort keys for determinism
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		_, _ = hasher.WriteString(k)
		_, _ = hasher.Write([]byte{'='})
		_, _ = hasher.WriteString(env[k])
		_, _ = hasher.Write([]byte{0})
	}
	_, _ = hasher.Write([]byte{0})
}

// hashInputFiles hashes the actual input files, handling globs and directories.
func (h *Hasher) hashInputFiles(task *domain.Task, root string, hasher *xxhash.Digest) error {
	for _, input := range task.Inputs {
		path := filepath.Join(root, input.String())

		if err := h.hashInputPath(path, hasher); err != nil {
			return err
		}
	}
	return nil
}

// hashInputPath hashes a single input path, attempting glob resolution if path doesn't exist.
func (h *Hasher) hashInputPath(path string, hasher *xxhash.Digest) error {
	_, err := os.Stat(path)
	if err != nil {
		return h.tryGlobAndHash(path, hasher)
	}
	return h.hashPath(path, hasher)
}

// tryGlobAndHash attempts to resolve a path as a glob pattern and hash all matches.
func (h *Hasher) tryGlobAndHash(path string, hasher *xxhash.Digest) error {
	matches, globErr := filepath.Glob(path)
	if globErr == nil && len(matches) > 0 {
		for _, match := range matches {
			if err := h.hashPath(match, hasher); err != nil {
				return err
			}
		}
		return nil
	}
	// If not a glob or no matches, return error as the input is missing
	return zerr.With(zerr.New("input not found"), "path", path)
}

func (h *Hasher) hashPath(path string, mainHasher io.Writer) error {
	info, err := os.Stat(path)
	if err != nil {
		return zerr.With(zerr.Wrap(err, "failed to stat path"), "path", path)
	}

	if info.IsDir() {
		// Use Walker to walk the directory
		// We pass empty ignores for now, or we could pass some default ignores.
		// The task might have ignores, but it's not in the struct yet.
		for filePath := range h.walker.WalkFiles(path, nil) {
			if err := h.hashFile(filePath, mainHasher); err != nil {
				return err
			}
		}
	} else {
		if err := h.hashFile(path, mainHasher); err != nil {
			return err
		}
	}
	return nil
}

func (h *Hasher) hashFile(path string, mainHasher io.Writer) error {
	// Write file path (relative or absolute? relative is better for cache portability, but here we just want uniqueness)
	// Let's write the path as is.
	_, _ = mainHasher.Write([]byte(path))
	_, _ = mainHasher.Write([]byte{0})

	// Compute file content hash
	hash, err := h.ComputeFileHash(path)
	if err != nil {
		return err
	}

	// Write hash to main hasher
	if err := binary.Write(mainHasher, binary.LittleEndian, hash); err != nil {
		return zerr.Wrap(err, "failed to write hash to digest")
	}
	return nil
}

// ComputeOutputHash computes the hash of the output files.
func (h *Hasher) ComputeOutputHash(outputs []string, root string) (string, error) {
	sortedOutputs := make([]string, len(outputs))
	copy(sortedOutputs, outputs)
	sort.Strings(sortedOutputs)

	hasher := xxhash.New()

	for _, output := range sortedOutputs {
		path := filepath.Join(root, output)

		// Check if file exists
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				return "", zerr.With(zerr.Wrap(iofs.ErrNotExist, "output file missing"), "path", path)
			}
			return "", zerr.With(zerr.Wrap(err, "failed to stat output file"), "path", path)
		}

		// Compute file hash
		hash, err := h.ComputeFileHash(path)
		if err != nil {
			return "", err
		}

		// Write hash to main hasher
		if err := binary.Write(hasher, binary.LittleEndian, hash); err != nil {
			return "", zerr.Wrap(err, "failed to write hash to digest")
		}
	}

	return fmt.Sprintf("%016x", hasher.Sum64()), nil
}
