package ports

import (
	"context"
	"io"
	"time"

	"go.trai.ch/same/internal/core/domain"
)

//go:generate mockgen -source=daemon.go -destination=mocks/mock_daemon.go -package=mocks

// DaemonStatus represents the current state of the daemon.
type DaemonStatus struct {
	Running       bool
	PID           int
	Uptime        time.Duration
	LastActivity  time.Time
	IdleRemaining time.Duration
}

// DaemonClient defines the interface for communicating with the daemon.
type DaemonClient interface {
	// Ping checks if the daemon is alive and resets the inactivity timer.
	Ping(ctx context.Context) error

	// Status returns the current daemon status.
	Status(ctx context.Context) (*DaemonStatus, error)

	// Shutdown requests a graceful daemon shutdown.
	Shutdown(ctx context.Context) error

	// GetGraph retrieves the task graph from the daemon.
	// configMtimes is a map of config file paths to their mtime (UnixNano).
	GetGraph(
		ctx context.Context,
		cwd string,
		configMtimes map[string]int64,
	) (graph *domain.Graph, cacheHit bool, err error)

	// GetEnvironment retrieves resolved Nix environment variables.
	GetEnvironment(
		ctx context.Context,
		envID string,
		tools map[string]string,
	) (envVars []string, cacheHit bool, err error)

	// GetInputHash retrieves the cached or pending input hash for a task.
	GetInputHash(
		ctx context.Context,
		taskName, root string,
		env map[string]string,
	) (InputHashResult, error)

	// ExecuteTask runs a task on the daemon and streams output.
	ExecuteTask(
		ctx context.Context,
		task *domain.Task,
		nixEnv []string,
		stdout, stderr io.Writer,
	) error

	// Close releases client resources.
	Close() error
}

// DaemonConnector manages daemon lifecycle from the CLI perspective.
type DaemonConnector interface {
	// Connect returns a client to the daemon, spawning it if necessary.
	// root is the workspace root directory where the daemon operates.
	Connect(ctx context.Context, root string) (DaemonClient, error)

	// IsRunning checks if the daemon process is currently running at the given root.
	IsRunning(root string) bool

	// Spawn starts a new daemon process in the background at the given root.
	Spawn(ctx context.Context, root string) error
}
