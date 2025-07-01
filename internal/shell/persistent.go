package shell

import (
	"sync"

	"github.com/charmbracelet/crush/internal/logging"
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

// loggingAdapter adapts the internal logging package to the Logger interface
type loggingAdapter struct{}

func (l *loggingAdapter) InfoPersist(msg string, keysAndValues ...interface{}) {
	logging.InfoPersist(msg, keysAndValues...)
}
