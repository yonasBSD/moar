package twin

type Logger interface {
	Debug(message string)

	// This level and up is recommended for adding to panic reports
	Info(message string)

	// If an error is logged, then something went wrong that the user should
	// know
	Error(message string)
}

type NoopLogger struct{}

func (l *NoopLogger) Debug(message string) {}

func (l *NoopLogger) Info(message string) {}

func (l *NoopLogger) Error(message string) {}

var log Logger = &NoopLogger{}
