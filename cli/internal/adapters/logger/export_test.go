// export_test.go exports private functions and types for white-box testing.
package logger

// ErrorEntry exports the private errorEntry fields for testing.
type ErrorEntry struct {
	Message  string
	Metadata map[string]any
}

// CollectErrorEntriesExported wraps collectErrorEntries and returns exported ErrorEntry types.
func CollectErrorEntriesExported(err error) []ErrorEntry {
	entries := collectErrorEntries(err)
	result := make([]ErrorEntry, len(entries))
	for i, e := range entries {
		result[i] = ErrorEntry{
			Message:  e.message,
			Metadata: e.metadata,
		}
	}
	return result
}

// FormatErrorEntriesExported wraps formatErrorEntries to accept exported ErrorEntry types.
func FormatErrorEntriesExported(entries []ErrorEntry) string {
	// Convert back to internal type
	internal := make([]errorEntry, len(entries))
	for i, e := range entries {
		internal[i] = errorEntry{
			message:  e.Message,
			metadata: e.Metadata,
		}
	}
	return formatErrorEntries(internal)
}
