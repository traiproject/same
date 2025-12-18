package ports

// InputResolver defines the interface for resolving input files.
//
//go:generate go run go.uber.org/mock/mockgen -destination=mocks/resolver_mock.go -package=mocks -source=resolver.go
type InputResolver interface {
	// ResolveInputs resolves the given input patterns to a list of concrete file paths.
	ResolveInputs(inputs []string, root string) ([]string, error)
}
