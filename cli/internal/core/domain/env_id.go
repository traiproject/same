package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"
)

// GenerateEnvID creates a deterministic hash from a tools map for environment caching.
func GenerateEnvID(tools map[string]string) string {
	// Sort keys for deterministic ordering
	aliases := make([]string, 0, len(tools))
	for alias := range tools {
		aliases = append(aliases, alias)
	}
	slices.Sort(aliases)

	// Build deterministic string
	var builder strings.Builder
	for _, alias := range aliases {
		spec := tools[alias]
		builder.WriteString(alias)
		builder.WriteString(":")
		builder.WriteString(spec)
		builder.WriteString(";")
	}

	// Hash the string
	hash := sha256.Sum256([]byte(builder.String()))
	return hex.EncodeToString(hash[:])
}
