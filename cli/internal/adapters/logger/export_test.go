// export_test.go exports private functions for white-box testing.
package logger

// ExportErrorFormatting exports the private error formatting functions for testing.
var (
	CollectErrorEntries = collectErrorEntries
	FormatErrorEntries  = formatErrorEntries
)
