package wiring_test

import (
	"testing"

	"github.com/grindlemire/graft"
)

// TestGraftDependencies ensures that the dependency injection graph is valid
// at compile/test time. It checks that every node declaring a dependency
// actually uses it, and every used dependency is declared.
func TestGraftDependencies(t *testing.T) {
	graft.AssertDepsValid(t, "../../internal")
}
