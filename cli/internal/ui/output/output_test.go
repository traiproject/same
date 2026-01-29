package output_test

import (
	"bytes"
	"testing"

	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"go.trai.ch/same/internal/ui/output"
)

func TestColorProfile(t *testing.T) {
	// Default
	t.Setenv("NO_COLOR", "")
	p := output.ColorProfile()
	assert.Equal(t, termenv.TrueColor, p)

	// No Color
	t.Setenv("NO_COLOR", "1")
	p = output.ColorProfile()
	assert.Equal(t, termenv.Ascii, p)
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
