package telemetry_test

import (
	"context"
	"sync"
	"time"
)

// mockRenderer is a simple test double for ports.Renderer.
type mockRenderer struct {
	mu            sync.Mutex
	planCalls     int
	startCalls    int
	logCalls      int
	completeCalls int
	logs          [][]byte
}

func (m *mockRenderer) Start(_ context.Context) error { return nil }
func (m *mockRenderer) Stop() error                   { return nil }
func (m *mockRenderer) Wait() error                   { return nil }

func (m *mockRenderer) OnPlanEmit(_ []string, _ map[string][]string, _ []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.planCalls++
}

func (m *mockRenderer) OnTaskStart(_, _, _ string, _ time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startCalls++
}

func (m *mockRenderer) OnTaskLog(_ string, data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logCalls++
	m.logs = append(m.logs, data)
}

func (m *mockRenderer) OnTaskComplete(_ string, _ time.Time, _ error, _ bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.completeCalls++
}
