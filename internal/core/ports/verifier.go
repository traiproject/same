package ports

// Verifier defines the interface for verifying file existence.
//
//go:generate mockgen -destination=mocks/verifier_mock.go -package=mocks -source=verifier.go
type Verifier interface {
	// VerifyOutputs checks if all output files exist in the given root directory.
	VerifyOutputs(root string, outputs []string) (bool, error)
}
