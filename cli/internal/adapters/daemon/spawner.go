package daemon

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"go.trai.ch/same/internal/core/domain"
	"go.trai.ch/same/internal/core/ports"
	"go.trai.ch/zerr"
)

const (
	pollInterval    = 100 * time.Millisecond
	maxPollDuration = 5 * time.Second
)

// Connector implements ports.DaemonConnector.
type Connector struct {
	executablePath string
}

// NewConnector creates a new daemon connector.
func NewConnector() (*Connector, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, zerr.Wrap(err, "failed to determine executable path")
	}
	return &Connector{executablePath: exe}, nil
}

// Connect returns a client, spawning the daemon if necessary.
func (c *Connector) Connect(ctx context.Context, root string) (ports.DaemonClient, error) {
	client, err := Dial(root)
	if err == nil {
		if pingErr := client.Ping(ctx); pingErr == nil {
			return client, nil
		}
		_ = client.Close()
	}

	if spawnErr := c.Spawn(ctx, root); spawnErr != nil {
		return nil, spawnErr
	}

	client, err = Dial(root)
	if err != nil {
		return nil, zerr.Wrap(err, "daemon client creation failed")
	}

	if pingErr := client.Ping(ctx); pingErr != nil {
		_ = client.Close()
		return nil, zerr.Wrap(pingErr, "daemon started but is not responsive")
	}

	return client, nil
}

// IsRunning checks if the daemon is running and responsive.
func (c *Connector) IsRunning(root string) bool {
	if root == "" {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	return c.isRunningWithCtx(ctx, root)
}

// isRunningWithCtx checks if the daemon is running and responsive, respecting the provided context.
func (c *Connector) isRunningWithCtx(ctx context.Context, root string) bool {
	client, err := Dial(root)
	if err != nil {
		return false
	}
	defer func() { _ = client.Close() }()

	if err := client.Ping(ctx); err != nil {
		return false
	}

	return true
}

// Spawn starts the daemon process in the background.
func (c *Connector) Spawn(ctx context.Context, root string) error {
	if root == "" {
		return zerr.New("root cannot be empty")
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return zerr.Wrap(err, "failed to resolve absolute root path")
	}

	daemonDir := filepath.Join(absRoot, filepath.Dir(domain.DefaultDaemonSocketPath()))
	if mkdirErr := os.MkdirAll(daemonDir, domain.DirPerm); mkdirErr != nil {
		return zerr.Wrap(mkdirErr, "failed to create daemon directory")
	}

	logPath := filepath.Join(absRoot, domain.DefaultDaemonLogPath())
	//nolint:gosec // G304: logPath is from root + domain constant, not user input
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, domain.PrivateFilePerm)
	if err != nil {
		return zerr.Wrap(err, "failed to open daemon log")
	}

	//nolint:gosec // G204: executablePath is controlled, args are fixed literals
	cmd := exec.Command(c.executablePath, "daemon", "serve")
	cmd.Dir = absRoot
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return zerr.Wrap(err, domain.ErrDaemonSpawnFailed.Error())
	}

	go func() {
		_ = cmd.Wait()
		_ = logFile.Close()
	}()

	if err := c.waitForDaemonStartup(ctx, absRoot); err != nil {
		return err
	}

	return nil
}

// waitForDaemonStartup waits for the daemon to become responsive.
func (c *Connector) waitForDaemonStartup(ctx context.Context, root string) error {
	start := time.Now()
	for time.Since(start) < maxPollDuration {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if c.isRunningWithCtx(ctx, root) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
		}
	}
	return zerr.New("daemon failed to start within timeout")
}
