package domain_test

import (
	"encoding/json"
	"testing"

	"go.trai.ch/same/internal/core/domain"
)

func TestInternedString(t *testing.T) {
	s1 := "hello"
	s2 := "hello"

	is1 := domain.NewInternedString(s1)
	is2 := domain.NewInternedString(s2)

	// Verify that the underlying handles are equal
	if is1.Value() != is2.Value() {
		t.Errorf("Expected handles to be equal for identical strings, got %v and %v", is1.Value(), is2.Value())
	}

	// Verify String() method
	if is1.String() != s1 {
		t.Errorf("Expected String() to return %q, got %q", s1, is1.String())
	}
}

func TestInternedStringJSON(t *testing.T) {
	t.Run("Marshal and Unmarshal preserve string value", func(t *testing.T) {
		original := domain.NewInternedString("test-task-name")

		// Marshal to JSON
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Failed to marshal InternedString: %v", err)
		}

		// Verify marshaled value
		expectedJSON := `"test-task-name"`
		if string(data) != expectedJSON {
			t.Errorf("Expected JSON %q, got %q", expectedJSON, string(data))
		}

		// Unmarshal from JSON
		var unmarshaled domain.InternedString
		err = json.Unmarshal(data, &unmarshaled)
		if err != nil {
			t.Fatalf("Failed to unmarshal InternedString: %v", err)
		}

		// Verify the string value is preserved
		if unmarshaled.String() != original.String() {
			t.Errorf("Expected unmarshaled string %q, got %q", original.String(), unmarshaled.String())
		}
	})

	t.Run("Marshal and Unmarshal in struct", func(t *testing.T) {
		type TestStruct struct {
			Name domain.InternedString `json:"name"`
		}

		original := TestStruct{
			Name: domain.NewInternedString("build"),
		}

		// Marshal to JSON
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Failed to marshal struct: %v", err)
		}

		// Verify marshaled value
		expectedJSON := `{"name":"build"}`
		if string(data) != expectedJSON {
			t.Errorf("Expected JSON %q, got %q", expectedJSON, string(data))
		}

		// Unmarshal from JSON
		var unmarshaled TestStruct
		err = json.Unmarshal(data, &unmarshaled)
		if err != nil {
			t.Fatalf("Failed to unmarshal struct: %v", err)
		}

		// Verify the string value is preserved
		if unmarshaled.Name.String() != original.Name.String() {
			t.Errorf("Expected unmarshaled name %q, got %q", original.Name.String(), unmarshaled.Name.String())
		}
	})
}

func TestNewInternedStrings(t *testing.T) {
	t.Run("Convert slice of strings to InternedStrings", func(t *testing.T) {
		strings := []string{"build", "test", "deploy"}

		internedStrings := domain.NewInternedStrings(strings)

		// Verify we got the correct number of elements
		if len(internedStrings) != len(strings) {
			t.Errorf("Expected %d interned strings, got %d", len(strings), len(internedStrings))
		}

		// Verify each string value is preserved
		for i, expected := range strings {
			if internedStrings[i].String() != expected {
				t.Errorf("Expected interned string at index %d to be %q, got %q", i, expected, internedStrings[i].String())
			}
		}
	})

	t.Run("Empty slice returns empty slice", func(t *testing.T) {
		emptyStrings := []string{}

		internedStrings := domain.NewInternedStrings(emptyStrings)

		if len(internedStrings) != 0 {
			t.Errorf("Expected empty slice, got %d elements", len(internedStrings))
		}
	})

	t.Run("Duplicate strings are deduplicated via interning", func(t *testing.T) {
		// Create same string multiple times to test interning
		s1 := "task"
		s2 := "task" // Same value

		internedStrings := domain.NewInternedStrings([]string{s1, s2})

		// Both should have equal handles due to interning
		if internedStrings[0].Value() != internedStrings[1].Value() {
			t.Errorf("Expected handles to be equal for identical strings")
		}
	})
}
