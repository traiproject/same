package logger_test

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"go.trai.ch/same/internal/adapters/logger"
	"go.trai.ch/zerr"
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

	// Assert that the output does NOT contain "INFO" (pretty format has no level prefix)
	if strings.Contains(output, "INFO") {
		t.Errorf("Expected output to NOT contain 'INFO' in pretty format, got: %s", output)
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

	// Assert that the output contains the error icon and message
	if !strings.Contains(output, "✗") {
		t.Errorf("Expected output to contain error icon '✗', got: %s", output)
	}

	// Assert that the output contains "Error:" prefix (new format)
	if !strings.Contains(output, "Error:") {
		t.Errorf("Expected output to contain 'Error:', got: %s", output)
	}

	// Assert that the error details are visible
	if !strings.Contains(output, "permission denied") {
		t.Errorf("Expected output to contain error details 'permission denied', got: %s", output)
	}

	// Assert that the output does NOT contain "ERROR" (pretty format has no level prefix)
	if strings.Contains(output, "ERROR") {
		t.Errorf("Expected output to NOT contain 'ERROR' in pretty format, got: %s", output)
	}
}

func TestLogger_Error_Wrapped(t *testing.T) {
	// Create a wrapped error chain using zerr.Wrap
	innerErr := errors.New("database connection failed")
	middleErr := zerr.Wrap(innerErr, "failed to load user data")
	outerErr := zerr.Wrap(middleErr, "failed to process request")

	// Capture stderr output
	output, err := captureStderr(func() {
		lg := logger.New()
		lg.Error(outerErr)
	})
	if err != nil {
		t.Fatalf("Failed to capture stderr: %v", err)
	}

	// Assert that the output contains the error icon
	if !strings.Contains(output, "✗") {
		t.Errorf("Expected output to contain error icon '✗', got: %s", output)
	}

	// Assert that the output contains "Error:" prefix
	if !strings.Contains(output, "Error:") {
		t.Errorf("Expected output to contain 'Error:', got: %s", output)
	}

	// Assert that the main error is displayed
	if !strings.Contains(output, "failed to process request") {
		t.Errorf("Expected output to contain main error 'failed to process request', got: %s", output)
	}

	// Assert that the cause chain is displayed
	if !strings.Contains(output, "Caused by:") {
		t.Errorf("Expected output to contain 'Caused by:', got: %s", output)
	}

	// Assert that all chain elements are displayed
	if !strings.Contains(output, "failed to load user data") {
		t.Errorf("Expected output to contain 'failed to load user data', got: %s", output)
	}

	if !strings.Contains(output, "database connection failed") {
		t.Errorf("Expected output to contain 'database connection failed', got: %s", output)
	}

	// Assert that the arrow is used for causes
	if !strings.Contains(output, "→") {
		t.Errorf("Expected output to contain arrow '→', got: %s", output)
	}
}

func TestLogger_Error_Multiline(t *testing.T) {
	// Create a multi-line error wrapped with zerr
	innerErr := errors.New("yaml: unmarshal errors:\n  line 30: cannot unmarshal !!str `go test...` into []string")
	wrappedErr := zerr.Wrap(innerErr, "failed to parse project config")

	// Capture stderr output
	output, err := captureStderr(func() {
		lg := logger.New()
		lg.Error(wrappedErr)
	})
	if err != nil {
		t.Fatalf("Failed to capture stderr: %v", err)
	}

	// Assert that the output contains the error icon
	if !strings.Contains(output, "✗") {
		t.Errorf("Expected output to contain error icon '✗', got: %s", output)
	}

	// Assert that the main error is displayed
	if !strings.Contains(output, "failed to parse project config") {
		t.Errorf("Expected output to contain main error 'failed to parse project config', got: %s", output)
	}

	// Assert that the multi-line content is preserved
	if !strings.Contains(output, "yaml") {
		t.Errorf("Expected output to contain 'yaml', got: %s", output)
	}

	if !strings.Contains(output, "line 30") {
		t.Errorf("Expected output to contain 'line 30', got: %s", output)
	}

	// Verify the hierarchical structure is shown
	if !strings.Contains(output, "Caused by:") {
		t.Errorf("Expected output to contain 'Caused by:', got: %s", output)
	}

	if !strings.Contains(output, "→") {
		t.Errorf("Expected output to contain arrow '→', got: %s", output)
	}
}

func TestLogger_Error_StandardWrapped(t *testing.T) {
	// Create a chain using fmt.Errorf (standard wrapping without zerr)
	innerErr := errors.New("connection refused")
	middleErr := fmt.Errorf("failed to connect to database: %w", innerErr)
	outerErr := fmt.Errorf("failed to initialize service: %w", middleErr)

	// Capture stderr output
	output, err := captureStderr(func() {
		lg := logger.New()
		lg.Error(outerErr)
	})
	if err != nil {
		t.Fatalf("Failed to capture stderr: %v", err)
	}

	// Assert that the output contains the error icon
	if !strings.Contains(output, "✗") {
		t.Errorf("Expected output to contain error icon '✗', got: %s", output)
	}

	// Assert that the output contains "Error:" prefix
	if !strings.Contains(output, "Error:") {
		t.Errorf("Expected output to contain 'Error:', got: %s", output)
	}

	// Assert that the full error message is displayed as a single line
	// (since fmt.Errorf doesn't implement messager, it falls back to Error())
	if !strings.Contains(output, "failed to initialize service") {
		t.Errorf("Expected output to contain 'failed to initialize service', got: %s", output)
	}

	// Verify that "Caused by:" is NOT shown for standard errors
	// (they are displayed as a single error message without hierarchy)
	if strings.Contains(output, "Caused by:") {
		t.Errorf("Expected output to NOT contain 'Caused by:' for standard errors, got: %s", output)
	}
}

func TestLogger_Error_Nil(t *testing.T) {
	// Test that calling Error with nil does not panic
	output, err := captureStderr(func() {
		lg := logger.New()
		lg.Error(nil)
	})
	if err != nil {
		t.Fatalf("Failed to capture stderr: %v", err)
	}

	// Assert that nothing was logged (empty output or just whitespace)
	trimmed := strings.TrimSpace(output)
	if trimmed != "" {
		t.Errorf("Expected no output for nil error, got: %s", output)
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

	// Assert that the output contains the warning icon and message
	if !strings.Contains(output, "!") {
		t.Errorf("Expected output to contain warning icon '!', got: %s", output)
	}

	if !strings.Contains(output, "some warning") {
		t.Errorf("Expected output to contain 'some warning', got: %s", output)
	}

	// Assert that the output does NOT contain "WARN" (pretty format has no level prefix)
	if strings.Contains(output, "WARN") {
		t.Errorf("Expected output to NOT contain 'WARN' in pretty format, got: %s", output)
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

func TestLogger_Error_WithMetadata(t *testing.T) {
	// Create an error with metadata using zerr.With
	baseErr := zerr.New("task definition is empty")
	metaErr := zerr.With(baseErr, "project", "cli")
	metaErr = zerr.With(metaErr, "task", "try")

	// Wrap it to create a chain
	outerErr := zerr.Wrap(metaErr, "failed to load configuration")

	// Capture stderr output
	output, err := captureStderr(func() {
		lg := logger.New()
		lg.Error(outerErr)
	})
	if err != nil {
		t.Fatalf("Failed to capture stderr: %v", err)
	}

	// Assert that the output contains the error icon
	if !strings.Contains(output, "✗") {
		t.Errorf("Expected output to contain error icon '✗', got: %s", output)
	}

	// Assert that the main error is displayed
	if !strings.Contains(output, "failed to load configuration") {
		t.Errorf("Expected output to contain 'failed to load configuration', got: %s", output)
	}

	// Assert that the cause chain is displayed
	if !strings.Contains(output, "Caused by:") {
		t.Errorf("Expected output to contain 'Caused by:', got: %s", output)
	}

	// Assert that the cause message is displayed
	if !strings.Contains(output, "task definition is empty") {
		t.Errorf("Expected output to contain 'task definition is empty', got: %s", output)
	}

	// Assert that metadata fields are displayed with proper indentation
	if !strings.Contains(output, "project: cli") {
		t.Errorf("Expected output to contain 'project: cli', got: %s", output)
	}

	if !strings.Contains(output, "task: try") {
		t.Errorf("Expected output to contain 'task: try', got: %s", output)
	}
}

func TestLogger_Error_WithPartialMetadata(t *testing.T) {
	// Create an error chain where only some errors have metadata
	innerErr := zerr.With(zerr.New("database timeout"), "timeout_ms", 5000)
	middleErr := zerr.Wrap(innerErr, "failed to fetch user") // No metadata
	outerErr := zerr.With(middleErr, "user_id", "12345")

	// Capture stderr output
	output, err := captureStderr(func() {
		lg := logger.New()
		lg.Error(outerErr)
	})
	if err != nil {
		t.Fatalf("Failed to capture stderr: %v", err)
	}

	// Assert main error has metadata
	if !strings.Contains(output, "user_id: 12345") {
		t.Errorf("Expected output to contain 'user_id: 12345', got: %s", output)
	}

	// Assert inner error has metadata
	if !strings.Contains(output, "timeout_ms: 5000") {
		t.Errorf("Expected output to contain 'timeout_ms: 5000', got: %s", output)
	}

	// Assert middle error doesn't have metadata lines
	// It should still show the message but no extra metadata
	if !strings.Contains(output, "failed to fetch user") {
		t.Errorf("Expected output to contain 'failed to fetch user', got: %s", output)
	}
}

func TestLogger_Error_MetadataSorting(t *testing.T) {
	// Create an error with metadata in non-alphabetical order
	baseErr := zerr.New("validation failed")
	// Add metadata in arbitrary order
	metaErr := zerr.With(baseErr, "zebra", "z")
	metaErr = zerr.With(metaErr, "alpha", "a")
	metaErr = zerr.With(metaErr, "mike", "m")

	// Capture stderr output
	output, err := captureStderr(func() {
		lg := logger.New()
		lg.Error(metaErr)
	})
	if err != nil {
		t.Fatalf("Failed to capture stderr: %v", err)
	}

	// Assert all metadata fields are present
	if !strings.Contains(output, "alpha: a") {
		t.Errorf("Expected output to contain 'alpha: a', got: %s", output)
	}
	if !strings.Contains(output, "mike: m") {
		t.Errorf("Expected output to contain 'mike: m', got: %s", output)
	}
	if !strings.Contains(output, "zebra: z") {
		t.Errorf("Expected output to contain 'zebra: z', got: %s", output)
	}

	// Verify they appear in sorted order (alpha, mike, zebra)
	alphaIdx := strings.Index(output, "alpha: a")
	mikeIdx := strings.Index(output, "mike: m")
	zebraIdx := strings.Index(output, "zebra: z")

	if alphaIdx == -1 || mikeIdx == -1 || zebraIdx == -1 {
		t.Fatalf("Could not find all expected metadata fields in output: %s", output)
	}

	if alphaIdx >= mikeIdx || mikeIdx >= zebraIdx {
		t.Errorf(
			"Expected metadata to be sorted alphabetically (alpha < mike < zebra), got indices: alpha=%d, mike=%d, zebra=%d",
			alphaIdx, mikeIdx, zebraIdx,
		)
	}
}

func TestLogger_Error_MainErrorWithMetadata(t *testing.T) {
	// Create an error where the main error (not just a cause) has metadata
	innerErr := errors.New("connection refused")
	outerErr := zerr.Wrap(innerErr, "service unavailable")
	// Attach metadata to the outer error
	outerErr = zerr.With(outerErr, "service", "auth-api")
	outerErr = zerr.With(outerErr, "retry_count", 3)

	// Capture stderr output
	output, err := captureStderr(func() {
		lg := logger.New()
		lg.Error(outerErr)
	})
	if err != nil {
		t.Fatalf("Failed to capture stderr: %v", err)
	}

	// Assert main error message is displayed
	if !strings.Contains(output, "service unavailable") {
		t.Errorf("Expected output to contain 'service unavailable', got: %s", output)
	}

	// Assert metadata is displayed on the main error with proper indentation (7 spaces)
	if !strings.Contains(output, "       retry_count: 3") {
		t.Errorf("Expected output to contain '       retry_count: 3' (7-space indent), got: %s", output)
	}

	if !strings.Contains(output, "       service: auth-api") {
		t.Errorf("Expected output to contain '       service: auth-api' (7-space indent), got: %s", output)
	}

	// Verify the cause chain is also shown
	if !strings.Contains(output, "Caused by:") {
		t.Errorf("Expected output to contain 'Caused by:', got: %s", output)
	}

	if !strings.Contains(output, "connection refused") {
		t.Errorf("Expected output to contain 'connection refused', got: %s", output)
	}
}
