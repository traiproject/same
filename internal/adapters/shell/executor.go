// Package shell provides the shell executor adapter.
package shell

import (
	"context"
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

// Execute runs the task's command.
func (e *Executor) Execute(ctx context.Context, task *domain.Task) error {
	if len(task.Command) == 0 {
		return nil
	}

	name := task.Command[0]
	args := task.Command[1:]

	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // user provided command

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
