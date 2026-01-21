package tui_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"go.trai.ch/same/internal/adapters/tui"
)

func TestColorProfile(t *testing.T) {
	// Default
	_ = os.Unsetenv("NO_COLOR")
	p := tui.ColorProfile()
	assert.Equal(t, termenv.TrueColor, p)

	// No Color
	_ = os.Setenv("NO_COLOR", "1")
	defer func() { _ = os.Unsetenv("NO_COLOR") }()
	p = tui.ColorProfile()
	assert.Equal(t, termenv.Ascii, p)
}

func TestNewOutput(t *testing.T) {
	var buf bytes.Buffer
	out := tui.NewOutput(&buf)
	assert.NotNil(t, out)

	_, _ = out.WriteString("test")
	assert.Equal(t, "test", buf.String())
}

func TestNewOutput_Nil(t *testing.T) {
	// Should default to stderr, we just check it doesn't panic
	out := tui.NewOutput(nil)
	assert.NotNil(t, out)
}
