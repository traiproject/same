package progrock_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.trai.ch/bob/internal/adapters/telemetry/progrock"
)

func TestNew(t *testing.T) {
	recorder := progrock.New()
	assert.NotNil(t, recorder)
}
