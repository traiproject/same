package logger_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.trai.ch/same/internal/adapters/logger"
)

func TestLogger(t *testing.T) {
	l := logger.New()

	// Swap output with a buffer
	var buf bytes.Buffer
	// Check if we can cast to *logger.Logger to set output
	// Since SetOutput is not in the interface, we need the concrete type
	concreteLogger, ok := l.(*logger.Logger)
	if !ok {
		t.Fatal("expected *logger.Logger")
	}
	concreteLogger.SetOutput(&buf)

	t.Run("Info", func(t *testing.T) {
		buf.Reset()
		l.Info("test info message")
		assert.Contains(t, buf.String(), "level=INFO")
		assert.Contains(t, buf.String(), "msg=\"test info message\"")
	})

	t.Run("Warn", func(t *testing.T) {
		buf.Reset()
		l.Warn("test warn message")
		assert.Contains(t, buf.String(), "level=WARN")
		assert.Contains(t, buf.String(), "msg=\"test warn message\"")
	})

	t.Run("Error", func(t *testing.T) {
		buf.Reset()
		err := errors.New("test error")
		l.Error(err)
		assert.Contains(t, buf.String(), "level=ERROR")
		assert.Contains(t, buf.String(), "msg=\"operation failed\"")
		assert.Contains(t, buf.String(), "error=\"test error\"")
	})
}
