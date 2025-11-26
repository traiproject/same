package ports

// Logger defines the interface for logging.
//
//go:generate mockgen -source=logger.go -destination=mocks/mock_logger.go -package=mocks
type Logger interface {
	Info(msg string)
	Error(err error)
}
