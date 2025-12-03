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
	var zero unique.Handle[string]
	if is.h == zero {
		return ""
	}
	return is.h.Value()
}

// Value returns the underlying unique.Handle[string].
func (is InternedString) Value() unique.Handle[string] {
	return is.h
}

// MarshalText implements encoding.TextMarshaler.
// It returns the bytes of the underlying string value.
func (is InternedString) MarshalText() ([]byte, error) {
	return []byte(is.h.Value()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
// It creates a new handle from the provided text.
func (is *InternedString) UnmarshalText(text []byte) error {
	is.h = unique.Make(string(text))
	return nil
}
