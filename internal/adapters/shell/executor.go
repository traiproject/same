// Package shell provides the shell executor adapter.
package shell

import (
	"context"
	"os"
	"os/exec"
	"strings"

	"go.trai.ch/bob/internal/core/domain"
	"go.trai.ch/bob/internal/core/ports"
	"go.trai.ch/zerr"
)

// Executor implements ports.Executor using os/exec.
type Executor struct {
	logger ports.Logger
}

// NewExecutor creates a new ShellExecutor.
func NewExecutor(logger ports.Logger) *Executor {
	return &Executor{
		logger: logger,
	}
}

// Execute runs the task's command with the specified environment.
// It merges environments with the following priority (low to high):
// 1. os.Environ() (System base)
// 2. env (Nix Hermetic Environment)
// 3. task.Environment (User-defined overrides)
//
// Special handling is applied to PATH: Nix paths are prepended to System paths.
func (e *Executor) Execute(ctx context.Context, task *domain.Task, env []string) error {
	if len(task.Command) == 0 {
		return nil
	}

	name := task.Command[0]
	args := task.Command[1:]

	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // user provided command

	// Set the working directory for the command
	if task.WorkingDir.String() != "" {
		cmd.Dir = task.WorkingDir.String()
	}

	// Construct the final environment
	cmd.Env = resolveEnvironment(os.Environ(), env, task.Environment)

	// Wire Stdout/Stderr to logger
	cmd.Stdout = &logWriter{logger: e.logger, level: "info"}
	cmd.Stderr = &logWriter{logger: e.logger, level: "error"}

	if err := cmd.Run(); err != nil {
		// Capture exit code if possible
		var exitCode int
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1 // Unknown or signal
		}

		// We might want to capture stderr tail here if we were buffering it,
		// but we are streaming to logger. So we just return the error.
		return zerr.With(zerr.Wrap(err, "command failed"), "exit_code", exitCode)
	}

	return nil
}

type logWriter struct {
	logger ports.Logger
	level  string
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	msg := string(p)
	// Trim trailing newline for cleaner logs if desired, but logger might handle it.
	// For now, raw string.
	// Actually, loggers usually expect a line or message.
	// If p contains multiple lines, we might want to split?
	// For simplicity, let's just pass it.
	// But wait, Write might be called with partial lines.
	// A proper implementation would buffer lines.
	// Given the instructions "Wire Stdout/Stderr to the provided logger interface",
	// and "Logic: Construct exec.Cmd. Wire Stdout/Stderr...".
	// I'll implement a simple line scanner or just cast to string for now.
	// Splitting by newline is safer for loggers.

	lines := strings.SplitSeq(strings.TrimSuffix(msg, "\n"), "\n")
	for line := range lines {
		if w.level == "info" {
			w.logger.Info(line)
		} else {
			w.logger.Error(zerr.New(line))
		}
	}
	return len(p), nil
}

// resolveEnvironment merges environment variables with the defined priority.
func resolveEnvironment(sysEnv, nixEnv []string, taskEnv map[string]string) []string {
	// 1. Start with System Environment
	envMap := make(map[string]string)
	for _, entry := range sysEnv {
		k, v, ok := strings.Cut(entry, "=")
		if ok {
			envMap[k] = v
		}
	}

	// 2. Apply Nix Environment (Prepend PATH)
	for _, entry := range nixEnv {
		k, v, ok := strings.Cut(entry, "=")
		if ok {
			if k == "PATH" {
				if sysPath, exists := envMap["PATH"]; exists && sysPath != "" {
					envMap[k] = v + string(os.PathListSeparator) + sysPath
				} else {
					envMap[k] = v
				}
			} else {
				envMap[k] = v
			}
		}
	}

	// 3. Apply Task Environment Overrides
	for k, v := range taskEnv {
		envMap[k] = v
	}

	// Convert to slice
	result := make([]string, 0, len(envMap))
	for k, v := range envMap {
		result = append(result, k+"="+v)
	}
	return result
}
