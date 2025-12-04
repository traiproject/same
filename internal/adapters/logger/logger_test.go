package logger_test

import (
	"io"
	"os"
	"strings"
	"testing"

	"go.trai.ch/bob/internal/adapters/logger"
)

// captureStderr captures output written to os.Stderr during the execution of fn.
func captureStderr(fn func()) (string, error) {
	// Save the original stderr
	originalStderr := os.Stderr

	// Create a pipe to capture stderr
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}

	// Replace os.Stderr with the write end of the pipe
	os.Stderr = w

	// Create a channel to signal when reading is complete
	done := make(chan string, 1)

	// Start reading in a goroutine
	go func() {
		buf, _ := io.ReadAll(r)
		done <- string(buf)
	}()

	// Execute the function
	fn()

	// Close the write end of the pipe to signal EOF to the reader
	if err := w.Close(); err != nil {
		os.Stderr = originalStderr
		return "", err
	}

	// Wait for the reading to complete
	output := <-done

	// Close the read end
	if err := r.Close(); err != nil {
		os.Stderr = originalStderr
		return "", err
	}

	// Restore the original stderr
	os.Stderr = originalStderr

	return output, nil
}

func TestLogger_Info(t *testing.T) {
	// Capture stderr output
	output, err := captureStderr(func() {
		// Create the logger inside the capture function so it uses the redirected stderr
		lg := logger.New()
		lg.Info("some message")
	})
	if err != nil {
		t.Fatalf("Failed to capture stderr: %v", err)
	}

	// Assert that the output contains "some message"
	if !strings.Contains(output, "some message") {
		t.Errorf("Expected output to contain 'some message', got: %s", output)
	}

	// Assert that the output contains "INFO"
	if !strings.Contains(output, "INFO") {
		t.Errorf("Expected output to contain 'INFO', got: %s", output)
	}
}

func TestLogger_Error(t *testing.T) {
	// Capture stderr output
	output, err := captureStderr(func() {
		// Create the logger inside the capture function so it uses the redirected stderr
		lg := logger.New()
		lg.Error(os.ErrPermission)
	})
	if err != nil {
		t.Fatalf("Failed to capture stderr: %v", err)
	}

	// Assert that the output contains the error message
	if !strings.Contains(output, "permission denied") {
		t.Errorf("Expected output to contain 'permission denied', got: %s", output)
	}

	// Assert that the output contains "ERROR"
	if !strings.Contains(output, "ERROR") {
		t.Errorf("Expected output to contain 'ERROR', got: %s", output)
	}
}

func TestLogger_Warn(t *testing.T) {
	// Capture stderr output
	output, err := captureStderr(func() {
		// Create the logger inside the capture function so it uses the redirected stderr
		lg := logger.New()
		lg.Warn("some warning")
	})
	if err != nil {
		t.Fatalf("Failed to capture stderr: %v", err)
	}

	// Assert that the output contains "some warning"
	if !strings.Contains(output, "some warning") {
		t.Errorf("Expected output to contain 'some warning', got: %s", output)
	}

	// Assert that the output contains "WARN"
	if !strings.Contains(output, "WARN") {
		t.Errorf("Expected output to contain 'WARN', got: %s", output)
	}
}

func TestNew(t *testing.T) {
	// Test that New() returns a non-nil logger
	lg := logger.New()

	if lg == nil {
		t.Fatal("Expected New() to return a non-nil logger")
	}

	// Test that the returned logger can be used
	// This test ensures the logger is properly initialized
	output, err := captureStderr(func() {
		// Create a fresh logger to ensure it uses the redirected stderr
		testLogger := logger.New()
		testLogger.Info("test initialization")
	})
	if err != nil {
		t.Fatalf("Failed to capture stderr: %v", err)
	}

	if !strings.Contains(output, "test initialization") {
		t.Errorf("Expected logger to log 'test initialization', got: %s", output)
	}
}
