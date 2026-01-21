package cas

// NewStoreWithPath exports newStoreWithPath for testing.
func NewStoreWithPath(path string) (*Store, error) {
	return newStoreWithPath(path)
}
