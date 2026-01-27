package daemon

import (
	"sync"
	"time"
)

// Lifecycle manages daemon inactivity timeout and shutdown.
type Lifecycle struct {
	mu           sync.Mutex
	timer        *time.Timer
	startTime    time.Time
	lastActivity time.Time
	timeout      time.Duration
	shutdownChan chan struct{}
	shutdownOnce sync.Once
}

// NewLifecycle creates a new lifecycle manager with the given timeout.
func NewLifecycle(timeout time.Duration) *Lifecycle {
	now := time.Now()
	l := &Lifecycle{
		startTime:    now,
		lastActivity: now,
		timeout:      timeout,
		shutdownChan: make(chan struct{}),
	}
	l.timer = time.AfterFunc(timeout, func() {
		l.triggerShutdown()
	})
	return l
}

// ResetTimer resets the inactivity timer. Called on every activity.
func (l *Lifecycle) ResetTimer() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lastActivity = time.Now()
	l.timer.Reset(l.timeout)
}

// IdleRemaining returns the duration until auto-shutdown.
func (l *Lifecycle) IdleRemaining() time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	elapsed := time.Since(l.lastActivity)
	remaining := l.timeout - elapsed
	if remaining < 0 {
		return 0
	}
	return remaining
}

// Uptime returns how long the daemon has been running.
func (l *Lifecycle) Uptime() time.Duration {
	return time.Since(l.startTime)
}

// LastActivity returns the timestamp of the last activity.
func (l *Lifecycle) LastActivity() time.Time {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.lastActivity
}

// ShutdownChan returns a channel that closes when shutdown is triggered.
func (l *Lifecycle) ShutdownChan() <-chan struct{} {
	return l.shutdownChan
}

// TriggerShutdown initiates shutdown (idempotent).
func (l *Lifecycle) triggerShutdown() {
	l.shutdownOnce.Do(func() {
		close(l.shutdownChan)
	})
}

// Shutdown stops the timer and triggers shutdown.
func (l *Lifecycle) Shutdown() {
	l.timer.Stop()
	l.triggerShutdown()
}
