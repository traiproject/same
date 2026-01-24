package linear_test

import (
	"testing"

	"go.trai.ch/same/internal/adapters/linear"
)

func TestNode_NewNode(t *testing.T) {
	node := linear.NewNode()
	if node == nil {
		t.Fatal("Expected non-nil node")
	}
}

func TestNode_Renderer(t *testing.T) {
	node := linear.NewNode()
	renderer := node.Renderer()
	if renderer == nil {
		t.Fatal("Expected non-nil renderer")
	}
}
