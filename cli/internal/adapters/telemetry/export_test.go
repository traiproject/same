package telemetry

// Batcher exposes the private batcher field for testing.
func (s *OTelSpan) Batcher() *BatchProcessor {
	return s.batcher
}
