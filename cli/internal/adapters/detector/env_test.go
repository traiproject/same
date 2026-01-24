package detector_test

import (
	"os"
	"testing"

	"go.trai.ch/same/internal/adapters/detector"
)

func TestDetectEnvironment(t *testing.T) {
	tests := []struct {
		name     string
		ciValue  string
		expected detector.OutputMode
	}{
		{
			name:     "CI=true forces linear mode",
			ciValue:  "true",
			expected: detector.ModeLinear,
		},
		{
			name:     "CI=1 forces linear mode",
			ciValue:  "1",
			expected: detector.ModeLinear,
		},
		{
			name:     "CI=false does not force linear",
			ciValue:  "false",
			expected: detector.ModeAuto,
		},
		{
			name:     "No CI env var",
			ciValue:  "",
			expected: detector.ModeAuto,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalCI := os.Getenv("CI")
			defer func() {
				if originalCI != "" {
					_ = os.Setenv("CI", originalCI)
				} else {
					_ = os.Unsetenv("CI")
				}
			}()

			if tt.ciValue != "" {
				if err := os.Setenv("CI", tt.ciValue); err != nil {
					t.Fatalf("Failed to set CI: %v", err)
				}
			} else {
				_ = os.Unsetenv("CI")
			}

			mode := detector.DetectEnvironment()

			if tt.ciValue == "true" || tt.ciValue == "1" {
				if mode != detector.ModeLinear {
					t.Errorf("Expected ModeLinear with CI=%s, got %v", tt.ciValue, mode)
				}
			}
		})
	}
}

func TestResolveMode(t *testing.T) {
	tests := []struct {
		name         string
		autoDetected detector.OutputMode
		userFlag     string
		expected     detector.OutputMode
	}{
		{
			name:         "auto respects auto-detection (TUI)",
			autoDetected: detector.ModeTUI,
			userFlag:     "auto",
			expected:     detector.ModeTUI,
		},
		{
			name:         "auto respects auto-detection (Linear)",
			autoDetected: detector.ModeLinear,
			userFlag:     "auto",
			expected:     detector.ModeLinear,
		},
		{
			name:         "empty flag respects auto-detection",
			autoDetected: detector.ModeTUI,
			userFlag:     "",
			expected:     detector.ModeTUI,
		},
		{
			name:         "tui overrides auto-detection",
			autoDetected: detector.ModeLinear,
			userFlag:     "tui",
			expected:     detector.ModeTUI,
		},
		{
			name:         "linear overrides auto-detection",
			autoDetected: detector.ModeTUI,
			userFlag:     "linear",
			expected:     detector.ModeLinear,
		},
		{
			name:         "ci is alias for linear",
			autoDetected: detector.ModeTUI,
			userFlag:     "ci",
			expected:     detector.ModeLinear,
		},
		{
			name:         "invalid flag respects auto-detection",
			autoDetected: detector.ModeTUI,
			userFlag:     "invalid",
			expected:     detector.ModeTUI,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detector.ResolveMode(tt.autoDetected, tt.userFlag)
			if got != tt.expected {
				t.Errorf("ResolveMode(%v, %q) = %v, want %v",
					tt.autoDetected, tt.userFlag, got, tt.expected)
			}
		})
	}
}

func TestResolveMode_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		autoDetected detector.OutputMode
		userFlag     string
		expected     detector.OutputMode
	}{
		{
			name:         "unknown flag falls back to auto-detection (Linear)",
			autoDetected: detector.ModeLinear,
			userFlag:     "unknown",
			expected:     detector.ModeLinear,
		},
		{
			name:         "empty string falls back to auto-detection (Linear)",
			autoDetected: detector.ModeLinear,
			userFlag:     "",
			expected:     detector.ModeLinear,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detector.ResolveMode(tt.autoDetected, tt.userFlag)
			if got != tt.expected {
				t.Errorf("ResolveMode(%v, %q) = %v, want %v",
					tt.autoDetected, tt.userFlag, got, tt.expected)
			}
		})
	}
}
