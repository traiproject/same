package domain

import (
	"testing"
)

func TestInternedString(t *testing.T) {
	s1 := "hello"
	s2 := "hello"

	is1 := NewInternedString(s1)
	is2 := NewInternedString(s2)

	// Verify that the underlying handles are equal
	if is1.Value() != is2.Value() {
		t.Errorf("Expected handles to be equal for identical strings, got %v and %v", is1.Value(), is2.Value())
	}

	// Verify String() method
	if is1.String() != s1 {
		t.Errorf("Expected String() to return %q, got %q", s1, is1.String())
	}
}
