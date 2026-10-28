package log

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/crush/internal/event"
	"github.com/charmbracelet/x/term"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	initOnce    sync.Once
	initialized atomic.Bool

	// file is the log file Setup writes to. SetFile moves it.
	file switchableFile
)

func Setup(logFile string, debug bool, ws ...io.Writer) {
	initOnce.Do(func() {
		_ = file.set(logFile)

		level := slog.LevelInfo
		if debug {
			level = slog.LevelDebug
		}

		opts := &slog.HandlerOptions{
			Level:     level,
			AddSource: true,
		}

		var handlers []slog.Handler
		handlers = append(handlers, slog.NewJSONHandler(&file, opts))

		for _, w := range ws {
			if w == nil {
				continue
			}
			if f, ok := w.(term.File); ok && term.IsTerminal(f.Fd()) {
				handlers = append(handlers, slog.NewTextHandler(w, opts))
			} else {
				handlers = append(handlers, slog.NewJSONHandler(w, opts))
			}
		}

		slog.SetDefault(slog.New(slog.NewMultiHandler(handlers...)))
		initialized.Store(true)
	})
}

func Initialized() bool {
	return initialized.Load()
}

// SetFile moves file logging to logFile. It closes the file written so far,
// so this process no longer holds it open. It does nothing before Setup.
func SetFile(logFile string) error {
	if !Initialized() {
		return nil
	}
	return file.set(logFile)
}

// switchableFile is a rotating log file that can be moved to another path
// while it is in use.
type switchableFile struct {
	mu sync.Mutex
	w  *lumberjack.Logger
}

func (f *switchableFile) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.w.Write(p)
}

// set points f at path and closes the file it wrote to before, if any.
func (f *switchableFile) set(path string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	prev := f.w
	f.w = &lumberjack.Logger{
		Filename:   path,
		MaxSize:    10,    // Max size in MB
		MaxBackups: 0,     // Number of backups
		MaxAge:     30,    // Days
		Compress:   false, // Enable compression
	}
	if prev == nil {
		return nil
	}
	return prev.Close()
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
