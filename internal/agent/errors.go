package agent

import "github.com/charmbracelet/crush/internal/errors"

// Re-export centralized errors for backward compatibility and convenient access
var (
	ErrRequestCancelled = errors.ErrRequestCancelled
	ErrSessionBusy      = errors.ErrSessionBusy
	ErrEmptyPrompt      = errors.ErrEmptyInput
	ErrSessionMissing   = errors.ErrSessionMissing
)
