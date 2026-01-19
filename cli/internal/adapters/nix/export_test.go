package nix

// GenerateNixExprForTest exports the private generateNixExpr method for testing purposes.
func (e *EnvFactory) GenerateNixExprForTest(system string, commits map[string][]string) string {
	return e.generateNixExpr(system, commits)
}
