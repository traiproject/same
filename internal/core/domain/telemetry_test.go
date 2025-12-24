package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.trai.ch/bob/internal/core/domain"
)

func TestVertexStatus(t *testing.T) {
	tests := []struct {
		name       string
		status     domain.VertexStatus
		isTerminal bool
	}{
		{"Pending", domain.VertexStatusPending, false},
		{"Running", domain.VertexStatusRunning, false},
		{"Completed", domain.VertexStatusCompleted, true},
		{"Failed", domain.VertexStatusFailed, true},
		{"Cached", domain.VertexStatusCached, true},
		{"Skipped", domain.VertexStatusSkipped, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.isTerminal, tt.status.IsTerminal())
		})
	}
}

func TestNormalizeVertexStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected domain.VertexStatus
	}{
		{"pending", domain.VertexStatusPending},
		{"PENDING", domain.VertexStatusPending},
		{"running", domain.VertexStatusRunning},
		{"completed", domain.VertexStatusCompleted},
		{"failed", domain.VertexStatusFailed},
		{"cached", domain.VertexStatusCached},
		{"skipped", domain.VertexStatusSkipped},
		{"unknown", domain.VertexStatusPending},
		{"", domain.VertexStatusPending},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, domain.NormalizeVertexStatus(tt.input))
		})
	}
}

func TestLogLevel_String(t *testing.T) {
	tests := []struct {
		level    domain.LogLevel
		expected string
	}{
		{domain.LogLevelDebug, "DEBUG"},
		{domain.LogLevelInfo, "INFO"},
		{domain.LogLevelWarn, "WARN"},
		{domain.LogLevelError, "ERROR"},
		{domain.LogLevel(999), "INFO"}, // Default case
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.level.String())
		})
	}
}
