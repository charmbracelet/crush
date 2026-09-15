// Package log is the project's leveled logger. Library code reports
// through it rather than printing directly.
package log

import (
	"fmt"
	"log"
	"os"
)

// Level is a severity.
type Level int

const (
	// LevelInfo is routine progress output.
	LevelInfo Level = iota
	// LevelWarn is a recoverable problem.
	LevelWarn
	// LevelError is a failure the caller decided to tolerate.
	LevelError
)

var std = log.New(os.Stderr, "", log.LstdFlags)

// Infof logs at LevelInfo.
func Infof(format string, args ...any) {
	std.Printf("INFO "+format, args...)
}

// Warnf logs at LevelWarn.
func Warnf(format string, args ...any) {
	std.Printf("WARN "+format, args...)
}

// Errorf logs at LevelError.
func Errorf(format string, args ...any) {
	std.Printf("ERROR "+format, args...)
}

// At logs at the given level.
func At(level Level, format string, args ...any) {
	switch level {
	case LevelInfo:
		Infof(format, args...)
	case LevelWarn:
		Warnf(format, args...)
	case LevelError:
		Errorf(format, args...)
	default:
		std.Print(fmt.Sprintf(format, args...))
	}
}
