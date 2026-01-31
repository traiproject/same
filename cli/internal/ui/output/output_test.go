package output_test

import (
	"bytes"
	"testing"

	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"go.trai.ch/same/internal/ui/output"
)

func TestColorProfile(t *testing.T) {
	// Test that NO_COLOR forces Ascii profile
	t.Setenv("NO_COLOR", "1")
	p := output.ColorProfile()
	assert.Equal(t, termenv.Ascii, p, "NO_COLOR should force Ascii profile")

	// Test that without NO_COLOR, EnvColorProfile is used
	// We don't assert the exact profile as it depends on the environment,
	// but we can verify the function works by calling it
	t.Setenv("NO_COLOR", "")
	p = output.ColorProfile()
	// Just verify it returns a valid profile (0-3)
	assert.True(t, p >= termenv.TrueColor && p <= termenv.Ascii, "should return a valid profile")
}

func TestColorProfileANSI(t *testing.T) {
	// Default
	t.Setenv("NO_COLOR", "")
	p := output.ColorProfileANSI()
	assert.Equal(t, termenv.ANSI, p)

	// No Color
	t.Setenv("NO_COLOR", "1")
	p = output.ColorProfileANSI()
	assert.Equal(t, termenv.Ascii, p)
}

func TestNew(t *testing.T) {
	var buf bytes.Buffer
	out := output.New(&buf)
	assert.NotNil(t, out)

	_, _ = out.WriteString("test")
	assert.Equal(t, "test", buf.String())
}

func TestNew_Nil(t *testing.T) {
	// Should default to stderr, we just check it doesn't panic
	out := output.New(nil)
	assert.NotNil(t, out)
}

func TestNewWithProfile(t *testing.T) {
	var buf bytes.Buffer
	out := output.NewWithProfile(&buf, output.ColorProfileANSI)
	assert.NotNil(t, out)

	_, _ = out.WriteString("test")
	assert.Equal(t, "test", buf.String())
}

func TestNewWithProfile_Nil(t *testing.T) {
	// Should default to stderr, we just check it doesn't panic
	out := output.NewWithProfile(nil, output.ColorProfile)
	assert.NotNil(t, out)
}
