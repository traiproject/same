package nix

// GenerateNixExprRaw exposes the internal generateNixExpr method for testing.
func (e *EnvFactory) GenerateNixExprRaw(system string, commits map[string][]string) string {
	return e.generateNixExpr(system, commits)
}
