package fs

import (
	"encoding/binary"
	"fmt"
	"io"
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
		return 0, zerr.With(zerr.Wrap(err, domain.ErrFileOpenFailed.Error()), "path", path)
	}
	defer f.Close() //nolint:errcheck // Best effort close in defer

	hasher := xxhash.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return 0, zerr.With(zerr.Wrap(err, domain.ErrFileHashFailed.Error()), "path", path)
	}

	return hasher.Sum64(), nil
}

// ComputeInputHash computes a single hash representing the task configuration,
// environment, and input files.
func (h *Hasher) ComputeInputHash(task *domain.Task, env map[string]string, inputs []string) (string, error) {
	hasher := xxhash.New()

	h.hashTaskDefinition(task, hasher)
	h.hashEnvironment(env, hasher)

	for _, path := range inputs {
		if err := h.hashPath(path, hasher); err != nil {
			return "", err
		}
	}

	return fmt.Sprintf("%016x", hasher.Sum64()), nil
}

// hashTaskDefinition hashes the task's name, command, inputs, outputs, and dependencies.
// Note: task.Inputs and task.Outputs are already canonicalized (sorted and deduplicated)
// by the configuration loader, so no additional sorting is needed here.
func (h *Hasher) hashTaskDefinition(task *domain.Task, hasher *xxhash.Digest) {
	// Name
	_, _ = hasher.WriteString(task.Name.String())
	_, _ = hasher.Write([]byte{0}) // Separator

	// Command
	for _, segment := range task.Command {
		_, _ = hasher.WriteString(segment)
		_, _ = hasher.Write([]byte{0})
	}
	_, _ = hasher.Write([]byte{0}) // Section separator

	// Tools
	// Sort keys for determinism
	toolKeys := make([]string, 0, len(task.Tools))
	for k := range task.Tools {
		toolKeys = append(toolKeys, k)
	}
	sort.Strings(toolKeys)

	for _, k := range toolKeys {
		_, _ = hasher.WriteString(k)
		_, _ = hasher.Write([]byte{0}) // Separator
		_, _ = hasher.WriteString(task.Tools[k])
		_, _ = hasher.Write([]byte{0}) // Separator
	}
	_, _ = hasher.Write([]byte{0}) // Section separator

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

func (h *Hasher) hashPath(path string, mainHasher io.Writer) error {
	info, err := os.Stat(path)
	if err != nil {
		return zerr.With(zerr.Wrap(err, domain.ErrPathStatFailed.Error()), "path", path)
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
		return zerr.Wrap(err, domain.ErrWriteHashFailed.Error())
	}
	return nil
}

// ComputeOutputHash computes the hash of the output files or directories.
// Note: Unlike task inputs/outputs, the output file list comes from filesystem traversal
// or executor results, which are not guaranteed to be in a deterministic order.
// Therefore, we must explicitly sort the list before hashing to ensure consistency.
func (h *Hasher) ComputeOutputHash(outputs []string, root string) (string, error) {
	sortedOutputs := make([]string, len(outputs))
	copy(sortedOutputs, outputs)
	sort.Strings(sortedOutputs)

	hasher := xxhash.New()

	for _, output := range sortedOutputs {
		path := filepath.Join(root, output)

		// Use hashPath to handle both files and directories
		if err := h.hashPath(path, hasher); err != nil {
			return "", err
		}
	}

	return fmt.Sprintf("%016x", hasher.Sum64()), nil
}
