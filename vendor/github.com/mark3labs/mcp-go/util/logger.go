package util

import (
	"log"
)

// Logger defines a minimal logging interface
type Logger interface {
	Infof(format string, v ...any)
	Errorf(format string, v ...any)
}

// --- Standard Library Logger Wrapper ---

// DefaultStdLogger implements Logger using the standard library's log.Logger.
func DefaultLogger() Logger {
	return &stdLogger{
		logger: log.Default(),
	}
}

// stdLogger wraps the standard library's log.Logger.
type stdLogger struct {
	logger *log.Logger
}

func (l *stdLogger) Infof(format string, v ...any) {
	l.logger.Printf("INFO: "+format, v...)
}

func (l *stdLogger) Errorf(format string, v ...any) {
	l.logger.Printf("ERROR: "+format, v...)
}
