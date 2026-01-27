package ports

import (
	"context"
	"time"
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

	// Close releases client resources.
	Close() error
}

// DaemonConnector manages daemon lifecycle from the CLI perspective.
type DaemonConnector interface {
	// Connect returns a client to the daemon, spawning it if necessary.
	Connect(ctx context.Context) (DaemonClient, error)

	// IsRunning checks if the daemon process is currently running.
	IsRunning() bool

	// Spawn starts a new daemon process in the background.
	Spawn(ctx context.Context) error
}
