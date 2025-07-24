package shell

import (
	"log/slog"
	"sync"
)

// PersistentShell is a singleton shell instance that maintains state across the application
type PersistentShell struct {
	*Shell
}

var (
	once          sync.Once
	shellInstance *PersistentShell
)

// GetPersistentShell returns the singleton persistent shell instance
// This maintains backward compatibility with the existing API
func GetPersistentShell(cwd string) *PersistentShell {
	once.Do(func() {
		shellInstance = &PersistentShell{
			Shell: NewShell(&Options{
				WorkingDir: cwd,
				Logger:     &loggingAdapter{},
			}),
		}
	})
	return shellInstance
}

// slog.dapter adapts the internal slog.package to the Logger interface
type loggingAdapter struct{}

func (l *loggingAdapter) InfoPersist(msg string, keysAndValues ...any) {
	slog.Info(msg, keysAndValues...)
}
