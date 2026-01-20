package domain_test

import (
	"testing"

	"go.trai.ch/same/internal/core/domain"
)

func TestGenerateEnvID_Deterministic(t *testing.T) {
	tools := map[string]string{"go": "go@1.25.4", "golangci-lint": "golangci-lint@2.6.2"}
	id1 := domain.GenerateEnvID(tools)
	id2 := domain.GenerateEnvID(tools)
	if id1 != id2 {
		t.Errorf("GenerateEnvID() not deterministic: %s != %s", id1, id2)
	}
}

func TestGenerateEnvID_EmptyMap(t *testing.T) {
	tools := map[string]string{}
	id1 := domain.GenerateEnvID(tools)
	if len(id1) != 64 {
		t.Errorf("GenerateEnvID() length = %d, want 64", len(id1))
	}
	// Empty map should produce consistent hash
	id2 := domain.GenerateEnvID(map[string]string{})
	if id1 != id2 {
		t.Errorf("GenerateEnvID() not deterministic for empty map")
	}
}

func TestGenerateEnvID_SingleTool(t *testing.T) {
	tools := map[string]string{"go": "go@1.25.4"}
	id := domain.GenerateEnvID(tools)
	if len(id) != 64 {
		t.Errorf("GenerateEnvID() length = %d, want 64 (SHA-256 hex)", len(id))
	}
}

func TestGenerateEnvID_OrderIndependent(t *testing.T) {
	tools1 := map[string]string{"go": "go@1.25.4", "golangci-lint": "golangci-lint@2.6.2"}
	tools2 := map[string]string{"golangci-lint": "golangci-lint@2.6.2", "go": "go@1.25.4"}
	id1 := domain.GenerateEnvID(tools1)
	id2 := domain.GenerateEnvID(tools2)
	if id1 != id2 {
		t.Errorf("GenerateEnvID() not order independent: %s != %s", id1, id2)
	}
}

func TestGenerateEnvID_DifferentTools(t *testing.T) {
	tools1 := map[string]string{"go": "go@1.25.4"}
	tools2 := map[string]string{"go": "go@1.24.0"}
	id1 := domain.GenerateEnvID(tools1)
	id2 := domain.GenerateEnvID(tools2)
	if id1 == id2 {
		t.Error("GenerateEnvID() produced same hash for different tools")
	}
}

func TestGenerateEnvID_HashFormat(t *testing.T) {
	tools := map[string]string{"go": "go@1.25.4", "node": "node@20.0.0", "python": "python@3.11.0"}
	id := domain.GenerateEnvID(tools)
	if len(id) != 64 {
		t.Errorf("GenerateEnvID() length = %d, want 64", len(id))
	}
	// Verify it's hexadecimal
	for _, c := range id {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			t.Errorf("GenerateEnvID() contains non-hex character: %c", c)
			break
		}
	}
}

func TestGenerateEnvID_SpecialCharacters(t *testing.T) {
	tools1 := map[string]string{"go": "nixpkgs#go@1.25", "rust": "nixpkgs#rust@1.70", "gcc": "nixpkgs#gcc@13.2"}
	tools2 := map[string]string{"go": "go@1.25", "rust": "rust@1.70", "gcc": "gcc@13.2"}
	id1 := domain.GenerateEnvID(tools1)
	id2 := domain.GenerateEnvID(tools2)
	if len(id1) != 64 {
		t.Errorf("GenerateEnvID() length = %d, want 64", len(id1))
	}
	// Ensure different from simpler version
	if id1 == id2 {
		t.Error("GenerateEnvID() should produce different hash for different specs")
	}
}
