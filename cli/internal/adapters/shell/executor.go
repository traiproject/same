// Package shell provides a shell-based executor for running tasks.
package shell

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/creack/pty"
	"go.trai.ch/same/internal/core/domain"
	"go.trai.ch/same/internal/core/ports"
	"go.trai.ch/zerr"
)

// Process represents a running command.
type Process interface {
	Wait() error
	Resize(rows, cols int) error
}

type ptyProcess struct {
	cmd    *exec.Cmd
	ptmx   *os.File
	ioDone <-chan struct{}
}

func (p *ptyProcess) Wait() error {
	// The pty.Start command starts the process.
	// We need to wait for it to finish.
	// Note: p.cmd.Wait() handles closing of some pipes, but for PTYs
	// we managed the ptmx.

	// Wait for the command to exit.
	err := p.cmd.Wait()

	// Wait for the IO copy loop to finish
	<-p.ioDone

	// Close the pty master if it hasn't been closed by the loop copying data.
	// Usually we close it after the command exits so that the copy loop finishes
	// reading what's left.

	return err
}

func (p *ptyProcess) Resize(rows, cols int) error {
	if rows > math.MaxUint16 || cols > math.MaxUint16 || rows < 0 || cols < 0 {
		return errors.New("terminal size out of bounds")
	}

	return pty.Setsize(p.ptmx, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
		X:    0,
		Y:    0,
	})
}

// Executor implements ports.Executor using os/exec and pty.
type Executor struct {
	logger ports.Logger
}

// NewExecutor creates a new ShellExecutor.
func NewExecutor(logger ports.Logger) *Executor {
	return &Executor{
		logger: logger,
	}
}

// Start launches the task's command in a PTY (on supported systems) or standard pipes.
// It returns a Process interface to control and wait for the command.
func (e *Executor) Start(
	ctx context.Context,
	task *domain.Task,
	env []string,
	stdout, stderr io.Writer,
) (Process, error) {
	// Combined writers:
	// 1. Structural Logger (info/error)
	// 2. Output Writers (Span, etc.)
	stdoutLog := &logWriter{logger: e.logger, level: "info"}
	stderrLog := &logWriter{logger: e.logger, level: "error"}

	finalStdout := io.MultiWriter(stdoutLog, stdout)
	finalStderr := io.MultiWriter(stderrLog, stderr)

	return start(ctx, task, env, finalStdout, finalStderr, stdoutLog, stderrLog)
}

func start(
	ctx context.Context,
	task *domain.Task,
	env []string,
	stdout, _ io.Writer,
	stdoutLog, stderrLog *logWriter,
) (Process, error) {
	if len(task.Command) == 0 {
		return nil, nil
	}

	name := task.Command[0]
	args := task.Command[1:]

	// Construct the final environment
	cmdEnv := resolveEnvironment(os.Environ(), env, task.Environment)

	// Resolve the executable path
	executable := name
	if !filepathIsAbs(name) {
		if lp, err := lookPath(name, cmdEnv); err == nil {
			executable = lp
		}
	}

	cmd := exec.CommandContext(ctx, executable, args...) //nolint:gosec // user provided command

	if len(cmd.Args) > 0 {
		cmd.Args[0] = name
	}

	if task.WorkingDir.String() != "" {
		cmd.Dir = task.WorkingDir.String()
	}

	cmd.Env = cmdEnv

	// pty.Start allows running with a PTY
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, zerr.Wrap(err, "failed to start pty")
	}

	ioDone := make(chan struct{})
	go func() {
		defer close(ioDone)
		defer func() { _ = ptmx.Close() }()
		// Ensure any remaining buffered logs are flushed when IO is done
		defer func() {
			_ = stdoutLog.Close()
			_ = stderrLog.Close()
		}()

		// Copy output to both stdout and stderr (since PTY merges them)
		// We use io.Copy which creates a 32k buffer. This is efficient enough.
		// The MultiWriter will ensure it goes to both logic logger and Span.
		_, _ = io.Copy(stdout, ptmx)
	}()

	return &ptyProcess{
		cmd:    cmd,
		ptmx:   ptmx,
		ioDone: ioDone,
	}, nil
}

// Execute runs the task's command and waits for it to complete.
func (e *Executor) Execute(ctx context.Context, task *domain.Task, env []string, stdout, stderr io.Writer) error {
	proc, err := e.Start(ctx, task, env, stdout, stderr)
	if err != nil {
		return err
	}
	if proc == nil {
		return nil // Empty command
	}

	// Mark execution start after process has started successfully
	if span, ok := stdout.(interface{ MarkExecStart() }); ok {
		span.MarkExecStart()
	}

	if err := proc.Wait(); err != nil {
		// Capture exit code if possible
		var exitCode int
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
		return zerr.With(zerr.Wrap(err, "command failed"), "exit_code", exitCode)
	}

	return nil
}

type logWriter struct {
	logger ports.Logger
	level  string
	buf    []byte
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	w.buf = append(w.buf, p...)

	// Scan for newlines
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}

		line := w.buf[:i]
		w.logLine(line)

		// Advance buffer
		w.buf = w.buf[i+1:]
	}

	return len(p), nil
}

func (w *logWriter) Close() error {
	if len(w.buf) > 0 {
		w.logLine(w.buf)
		w.buf = nil
	}
	return nil
}

func (w *logWriter) logLine(line []byte) {
	msg := string(line)
	// PTYs may introduce \r. Remove it.
	msg = strings.TrimSuffix(msg, "\r")

	if w.level == "info" {
		w.logger.Info(msg)
	} else {
		w.logger.Error(zerr.New(msg))
	}
}

// allowListedEnvVars are the system environment variables that are allowed to be
// inherited by the task. This ensures the build environment is hermetic and
// reproducible, while still allowing basic system tools to function.
var allowListedEnvVars = map[string]struct{}{
	"HOME": {},
	"TERM": {},
	"USER": {},
	"PATH": {},
}

// resolveEnvironment merges environment variables with the defined priority.
func resolveEnvironment(sysEnv, nixEnv []string, taskEnv map[string]string) []string {
	// 1. Start with System Environment (Allow-list only)
	envMap := filterSystemEnv(sysEnv)

	// 2. Apply Nix Environment (Prepend PATH)
	applyNixEnv(envMap, nixEnv)

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

func filterSystemEnv(sysEnv []string) map[string]string {
	envMap := make(map[string]string)
	for _, entry := range sysEnv {
		k, v, ok := strings.Cut(entry, "=")
		if ok {
			if _, allowed := allowListedEnvVars[k]; allowed {
				envMap[k] = v
			}
		}
	}
	return envMap
}

func applyNixEnv(envMap map[string]string, nixEnv []string) {
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
}

// lookPath searches for an executable in the directories named by the PATH environment variable.
func lookPath(file string, env []string) (string, error) {
	// Find PATH in env
	var path string
	for _, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			path = strings.TrimPrefix(e, "PATH=")
			break
		}
	}

	if path == "" {
		return "", exec.ErrNotFound
	}

	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			// Unix shell semantics: path element "" means "."
			dir = "."
		}
		path := filepath.Join(dir, file)
		if err := findExecutable(path); err == nil {
			return path, nil
		}
	}
	return "", exec.ErrNotFound
}

func findExecutable(file string) error {
	d, err := os.Stat(file)
	if err != nil {
		return err
	}
	if m := d.Mode(); !m.IsDir() && m&0o111 != 0 {
		return nil
	}
	return os.ErrPermission
}

// filepathIsAbs checks if a path is absolute.
func filepathIsAbs(path string) bool {
	return filepath.IsAbs(path)
}
