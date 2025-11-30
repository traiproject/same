package nix

import (
	"fmt"
	"strings"
)

// generateNixExpression creates a Nix expression that defines a shell environment
// with the specified dependencies.
func generateNixExpression(dependencies []string) string {
	if len(dependencies) == 0 {
		return "let pkgs = import <nixpkgs> {}; in pkgs.mkShell { buildInputs = []; }"
	}

	// Sanitize and quote dependencies to prevent injection
	quotedDeps := make([]string, len(dependencies))
	for i, dep := range dependencies {
		quotedDeps[i] = fmt.Sprintf("%q", dep)
	}

	// We use a simple string builder for the expression.
	// In a more complex scenario, we might use a template.
	// We map the string dependencies to actual packages in the expression.
	// Assuming dependencies are package names available in nixpkgs.
	// For simplicity, we'll use `pkgs.lib.attrByPath` or just assume top-level or legacy support.
	// However, `nix-shell -p` usually takes package names.
	// Inside an expression, we usually refer to them as `pkgs.packageName`.
	// But the user request says "list of package names".
	// Let's construct a list of packages looked up from pkgs.

	// A robust way is to use `with import <nixpkgs> {}; mkShell { buildInputs = [ dep1 dep2 ... ]; }`
	// But we need to ensure `dep1` is treated as an attribute lookup.
	// If the user passes "git", we want `pkgs.git`.
	depsString := strings.Join(dependencies, " ")
	return fmt.Sprintf("let pkgs = import <nixpkgs> {}; in pkgs.mkShell { buildInputs = with pkgs; [ %s ]; }", depsString)
}
