package crush

import "github.com/charmbracelet/crush/internal/session"

type (
	Session        = session.Session
	Todo           = session.Todo
	TodoStatus     = session.TodoStatus
	SessionService = session.Service
)

const (
	TodoStatusPending    = session.TodoStatusPending
	TodoStatusInProgress = session.TodoStatusInProgress
	TodoStatusCompleted  = session.TodoStatusCompleted
)
