package domain

import "unique"

// InternedString is a value object that wraps a unique.Handle[string].
// It is used to reduce memory usage for frequently repeated strings like task names and file paths.
type InternedString struct {
	h unique.Handle[string]
}

// NewInternedString creates a new InternedString from a string.
// It uses the unique package to intern the string.
func NewInternedString(s string) InternedString {
	return InternedString{
		h: unique.Make(s),
	}
}

// String returns the underlying string value.
func (is InternedString) String() string {
	return is.h.Value()
}

// Value returns the underlying unique.Handle[string].
func (is InternedString) Value() unique.Handle[string] {
	return is.h
}
