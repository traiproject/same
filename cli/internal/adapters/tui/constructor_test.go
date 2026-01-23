package tui_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.trai.ch/same/internal/adapters/tui"
)

func TestNewModel(t *testing.T) {
	m := tui.NewModel(nil)

	assert.NotNil(t, m.Tasks)
	assert.Empty(t, m.Tasks)
	assert.NotNil(t, m.TaskMap)
	assert.Empty(t, m.TaskMap)
	assert.NotNil(t, m.SpanMap)
	assert.Empty(t, m.SpanMap)
	assert.True(t, m.AutoScroll, "AutoScroll should be true by default")
}

func TestNewModel_WithWriter(t *testing.T) {
	buf := &bytes.Buffer{}
	m := tui.NewModel(buf)

	assert.NotNil(t, m.Output)
	assert.True(t, m.FollowMode)
	assert.Equal(t, tui.ViewModeTree, m.ViewMode)
}

func TestModel_WithDisableTick(t *testing.T) {
	m := tui.NewModel(nil)
	assert.False(t, m.DisableTick)

	m = m.WithDisableTick()
	assert.True(t, m.DisableTick)
}
