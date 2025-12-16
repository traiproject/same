package ports

// Logger defines the interface for logging.
//
//go:generate go run go.uber.org/mock/mockgen -source=logger.go -destination=mocks/mock_logger.go -package=mocks
type Logger interface {
	Info(msg string)
	Warn(msg string)
	Error(err error)
}
