package log

import (
	"log"
	"strings"
)

type stdLogWriter struct {
	l   *Logger
	opt *StandardLogOptions
}

func (l *stdLogWriter) Write(p []byte) (n int, err error) {
	str := strings.TrimSuffix(string(p), "\n")

	if l.opt != nil {
		switch l.opt.ForceLevel {
		case DebugLevel:
			l.l.Debug(str)
		case InfoLevel:
			l.l.Info(str)
		case WarnLevel:
			l.l.Warn(str)
		case ErrorLevel:
			l.l.Error(str)
		}
	} else {
		switch {
		case strings.HasPrefix(str, "DEBUG"):
			l.l.Debug(strings.TrimSpace(str[5:]))
		case strings.HasPrefix(str, "INFO"):
			l.l.Info(strings.TrimSpace(str[4:]))
		case strings.HasPrefix(str, "WARN"):
			l.l.Warn(strings.TrimSpace(str[4:]))
		case strings.HasPrefix(str, "ERROR"):
			l.l.Error(strings.TrimSpace(str[5:]))
		case strings.HasPrefix(str, "ERR"):
			l.l.Error(strings.TrimSpace(str[3:]))
		default:
			l.l.Info(str)
		}
	}

	return len(p), nil
}

// StandardLogOptions can be used to configure the standard log adapter.
type StandardLogOptions struct {
	ForceLevel Level
}

// StandardLog returns a standard logger from Logger. The returned logger
// can infer log levels from message prefix. Expected prefixes are DEBUG, INFO,
// WARN, ERROR, and ERR.
func (l *Logger) StandardLog(opts ...StandardLogOptions) *log.Logger {
	nl := l.With()
	// The caller stack is
	// log.Printf() -> l.Output() -> l.out.Write(stdLogger.Write)
	nl.callerOffset += 3
	sl := &stdLogWriter{
		l: nl,
	}
	if len(opts) > 0 {
		sl.opt = &opts[0]
	}
	return log.New(sl, "", 0)
}
