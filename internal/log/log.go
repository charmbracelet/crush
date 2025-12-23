package log

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/crush/internal/event"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	initOnce    sync.Once
	initialized atomic.Bool
)

const MaxAgeDays = 30

func Setup(logFile string, debug bool) {
	initOnce.Do(func() {
		// Create a process-specific log file name to avoid conflicts between multiple processes
		pid := os.Getpid()
		dir := filepath.Dir(logFile)
		ext := filepath.Ext(logFile)
		name := strings.TrimSuffix(filepath.Base(logFile), ext)
		processLogFile := filepath.Join(dir, fmt.Sprintf("%s-%d%s", name, pid, ext))

		logRotator := &lumberjack.Logger{
			Filename:   processLogFile,
			MaxSize:    10,         // Max size in MB
			MaxBackups: 0,          // Number of backups
			MaxAge:     MaxAgeDays, // Days
			Compress:   false,      // Enable compression
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

func RecoverPanic(name string, cleanup func()) {
	if r := recover(); r != nil {
		event.Error(r, "panic", true, "name", name)

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
