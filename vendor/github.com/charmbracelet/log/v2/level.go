package log

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

// Level is a logging level.
type Level int

const (
	// DebugLevel is the debug level.
	DebugLevel Level = -4
	// InfoLevel is the info level.
	InfoLevel Level = 0
	// WarnLevel is the warn level.
	WarnLevel Level = 4
	// ErrorLevel is the error level.
	ErrorLevel Level = 8
	// FatalLevel is the fatal level.
	FatalLevel Level = 12
	// noLevel is used with log.Print.
	noLevel Level = math.MaxInt
)

// String returns the string representation of the level.
func (l Level) String() string {
	switch l {
	case DebugLevel:
		return "debug"
	case InfoLevel:
		return "info"
	case WarnLevel:
		return "warn"
	case ErrorLevel:
		return "error"
	case FatalLevel:
		return "fatal"
	default:
		return ""
	}
}

// ErrInvalidLevel is an error returned when parsing an invalid level string.
var ErrInvalidLevel = errors.New("invalid level")

// ParseLevel converts level in string to Level type. Default level is InfoLevel.
func ParseLevel(level string) (Level, error) {
	switch strings.ToLower(level) {
	case DebugLevel.String():
		return DebugLevel, nil
	case InfoLevel.String():
		return InfoLevel, nil
	case WarnLevel.String():
		return WarnLevel, nil
	case ErrorLevel.String():
		return ErrorLevel, nil
	case FatalLevel.String():
		return FatalLevel, nil
	default:
		return 0, fmt.Errorf("%w: %q", ErrInvalidLevel, level)
	}
}
