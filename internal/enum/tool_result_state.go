package enum

import (
	"image/color"

	"github.com/charmbracelet/crush/internal/tui/styles"
)

// ToolResultState represents the result state of tool execution.
// This replaces boolean IsError with type-safe result states.
type ToolResultState uint8

const (
	// ToolResultStateUnknown indicates indeterminate or unknown result state
	ToolResultStateUnknown ToolResultState = iota

	// ToolResultStateError indicates tool execution failed with error
	ToolResultStateError

	// ToolResultStateTimeout indicates tool execution timed out
	ToolResultStateTimeout

	// ToolResultStateCancelled indicates tool was cancelled before completion
	ToolResultStateCancelled

	// ToolResultStatePartial indicates partial success (some operations succeeded)
	ToolResultStatePartial

	// ToolResultStateSuccess indicates successful tool execution
	ToolResultStateSuccess
)

// IsSuccess returns true if the result state indicates successful execution
func (state ToolResultState) IsSuccess() bool {
	return state == ToolResultStateSuccess || state == ToolResultStatePartial
}

// IsError returns true if the result state indicates an error
func (state ToolResultState) IsError() bool {
	return state == ToolResultStateError || state == ToolResultStateTimeout || state == ToolResultStateUnknown
}

// IsFinal returns true if the result state is final (no further processing expected)
func (state ToolResultState) IsFinal() bool {
	return state != ToolResultStateUnknown
}

// ToIcon returns appropriate icon for result state
func (state ToolResultState) ToIcon() string {
	switch state {
	case ToolResultStateSuccess:
		return styles.ToolSuccess
	case ToolResultStateError:
		return styles.ToolError
	case ToolResultStateTimeout:
		return "⏱️" // Timeout icon
	case ToolResultStateCancelled:
		return styles.ToolCancel
	case ToolResultStatePartial:
		return "⚠️" // Warning icon for partial success
	case ToolResultStateUnknown:
		return "❓" // Question mark for unknown
	default:
		return styles.ToolError // Fallback to error icon
	}
}

// ToFgColor returns appropriate foreground color for result state
func (state ToolResultState) ToFgColor() color.Color {
	t := styles.CurrentTheme()
	switch state {
	case ToolResultStateSuccess:
		return t.Green // Bright green for success
	case ToolResultStateError:
		return t.Error // Red for errors
	case ToolResultStateTimeout:
		return t.Paprika // Orange for timeouts
	case ToolResultStateCancelled:
		return t.FgMuted // Grey for cancellation
	case ToolResultStatePartial:
		return t.Paprika // Orange for partial success
	case ToolResultStateUnknown:
		return t.FgSubtle // Subtle for unknown
	default:
		return t.Error // Fallback to error color
	}
}

// ToIconColored returns colored icon for result state
func (state ToolResultState) ToIconColored() string {
	t := styles.CurrentTheme()
	return t.S().Base.Foreground(state.ToFgColor()).Render(state.ToIcon())
}

// String returns string representation of result state
func (state ToolResultState) String() string {
	switch state {
	case ToolResultStateSuccess:
		return "success"
	case ToolResultStateError:
		return "error"
	case ToolResultStateTimeout:
		return "timeout"
	case ToolResultStateCancelled:
		return "cancelled"
	case ToolResultStatePartial:
		return "partial"
	case ToolResultStateUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}

// ToLabel returns descriptive label for result state
func (state ToolResultState) ToLabel() string {
	switch state {
	case ToolResultStateSuccess:
		return "Success"
	case ToolResultStateError:
		return "Error"
	case ToolResultStateTimeout:
		return "Timeout"
	case ToolResultStateCancelled:
		return "Cancelled"
	case ToolResultStatePartial:
		return "Partial Success"
	case ToolResultStateUnknown:
		return "Unknown"
	default:
		return "Unknown"
	}
}

// RenderTUIMessage returns message for TUI display
func (state ToolResultState) RenderTUIMessage() string {
	switch state {
	case ToolResultStateSuccess:
		return "Completed successfully"
	case ToolResultStateError:
		return "Tool execution failed"
	case ToolResultStateTimeout:
		return "Tool execution timed out"
	case ToolResultStateCancelled:
		return "Tool execution cancelled"
	case ToolResultStatePartial:
		return "Partially completed"
	case ToolResultStateUnknown:
		return "Result unknown"
	default:
		return "Unknown result state"
	}
}
