package tui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.trai.ch/bob/internal/adapters/tui"
)

func TestNewModel(t *testing.T) {
	m := tui.NewModel()

	assert.NotNil(t, m.Tasks)
	assert.Empty(t, m.Tasks)
	assert.NotNil(t, m.TaskMap)
	assert.Empty(t, m.TaskMap)
	assert.NotNil(t, m.SpanMap)
	assert.Empty(t, m.SpanMap)
	assert.NotNil(t, m.Viewport)
	assert.True(t, m.AutoScroll, "AutoScroll should be true by default")
}
