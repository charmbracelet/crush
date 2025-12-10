package enum

import "github.com/charmbracelet/crush/internal/errors"

// Re-export centralized errors for backward compatibility and convenient access
var (
	ErrToolCallStateUnknown   = errors.ErrToolCallStateUnknown
	ErrAnimationStateUnknown  = errors.ErrAnimationStateUnknown
	ErrToolResultStateUnknown = errors.ErrToolResultStateUnknown
)
