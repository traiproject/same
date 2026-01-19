package domain_test

import (
	"path/filepath"
	"testing"

	"go.trai.ch/same/internal/core/domain"
)

func TestLayoutPaths(t *testing.T) {
	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{
			name:     "DefaultSamePath",
			got:      domain.DefaultSamePath(),
			expected: ".same",
		},
		{
			name:     "DefaultStorePath",
			got:      domain.DefaultStorePath(),
			expected: filepath.Join(".same", "store"),
		},
		{
			name:     "DefaultNixHubCachePath",
			got:      domain.DefaultNixHubCachePath(),
			expected: filepath.Join(".same", "cache", "nixhub"),
		},
		{
			name:     "DefaultEnvCachePath",
			got:      domain.DefaultEnvCachePath(),
			expected: filepath.Join(".same", "cache", "environments"),
		},
		{
			name:     "DefaultDebugLogPath",
			got:      domain.DefaultDebugLogPath(),
			expected: filepath.Join(".same", "debug.log"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("%s() = %v, want %v", tt.name, tt.got, tt.expected)
			}
		})
	}
}
