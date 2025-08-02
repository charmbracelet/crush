package log

import (
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	initOnce    sync.Once
	initialized atomic.Bool
)

func Setup(logFile string, debug bool) {
	initOnce.Do(func() {
		logRotator := &lumberjack.Logger{
			Filename:   logFile,
			MaxSize:    10,    // Max size in MB
			MaxBackups: 0,     // Number of backups
			MaxAge:     30,    // Days
			Compress:   false, // Enable compression
		}

		level := slog.LevelInfo
		if debug {
			level = slog.LevelDebug
		}

		logger := slog.NewJSONHandler(logRotator, &slog.HandlerOptions{
			Level:     level,
			AddSource: true,
		})

		slog.SetDefault(slog.New(logger))
		initialized.Store(true)
	})
}

func Initialized() bool {
	return initialized.Load()
}

// MaskAPIKey masks an API key by showing only the first and last 5 characters.
// For keys shorter than 10 characters, it shows first 2 and last 2 characters.
// Returns "***EMPTY***" for empty strings.
func MaskAPIKey(apiKey string) string {
	if apiKey == "" {
		return "***EMPTY***"
	}

	// Remove common prefixes
	key := strings.TrimPrefix(apiKey, "Bearer ")
	key = strings.TrimPrefix(key, "sk-")

	keyLen := len(key)
	if keyLen <= 4 {
		return strings.Repeat("*", keyLen)
	} else if keyLen <= 10 {
		return key[:2] + strings.Repeat("*", keyLen-4) + key[keyLen-2:]
	} else {
		return key[:5] + strings.Repeat("*", keyLen-10) + key[keyLen-5:]
	}
}

func RecoverPanic(name string, cleanup func()) {
	if r := recover(); r != nil {
		// Create a timestamped panic log file
		timestamp := time.Now().Format("20060102-150405")
		filename := fmt.Sprintf("crush-panic-%s-%s.log", name, timestamp)

		file, err := os.Create(filename)
		if err == nil {
			defer file.Close()

			// Write panic information and stack trace
			fmt.Fprintf(file, "Panic in %s: %v\n\n", name, r)
			fmt.Fprintf(file, "Time: %s\n\n", time.Now().Format(time.RFC3339))
			fmt.Fprintf(file, "Stack Trace:\n%s\n", debug.Stack())

			// Execute cleanup function if provided
			if cleanup != nil {
				cleanup()
			}
		}
	}
}
