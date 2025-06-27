package logging

import (
	"fmt"
	"os"
	"runtime/debug"
	"time"

	"github.com/charmbracelet/log"
)

func Info(msg string, args ...any) {
	log.Info(msg, args...)
}

func Debug(msg string, args ...any) {
	log.Debug(msg, args...)
}

func Warn(msg string, args ...any) {
	log.Warn(msg, args...)
}

func Error(msg string, args ...any) {
	log.Error(msg, args...)
}

func InfoPersist(msg string, args ...any) {
	args = append(args, persistKeyArg, true)
	log.Info(msg, args...)
}

func DebugPersist(msg string, args ...any) {
	args = append(args, persistKeyArg, true)
	log.Debug(msg, args...)
}

func WarnPersist(msg string, args ...any) {
	args = append(args, persistKeyArg, true)
	log.Warn(msg, args...)
}

func ErrorPersist(msg string, args ...any) {
	args = append(args, persistKeyArg, true)
	log.Error(msg, args...)
}

// RecoverPanic is a common function to handle panics gracefully.
// It logs the error, creates a panic log file with stack trace,
// and executes an optional cleanup function before returning.
func RecoverPanic(name string, cleanup func()) {
	if r := recover(); r != nil {
		// Log the panic
		ErrorPersist(fmt.Sprintf("Panic in %s: %v", name, r))

		// Create a timestamped panic log file
		timestamp := time.Now().Format("20060102-150405")
		filename := fmt.Sprintf("crush-panic-%s-%s.log", name, timestamp)

		file, err := os.Create(filename)
		if err != nil {
			ErrorPersist(fmt.Sprintf("Failed to create panic log: %v", err))
		} else {
			defer file.Close()

			// Write panic information and stack trace
			fmt.Fprintf(file, "Panic in %s: %v\n\n", name, r)
			fmt.Fprintf(file, "Time: %s\n\n", time.Now().Format(time.RFC3339))
			fmt.Fprintf(file, "Stack Trace:\n%s\n", debug.Stack())

			InfoPersist(fmt.Sprintf("Panic details written to %s", filename))
		}

		// Execute cleanup function if provided
		if cleanup != nil {
			cleanup()
		}
	}
}
