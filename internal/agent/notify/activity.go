package notify

import (
	"time"

	"github.com/charmbracelet/crush/internal/message"
)

// RunActivityType identifies a user-visible phase of an agent run.
type RunActivityType string

const (
	RunActivityThinkingStarted  RunActivityType = "thinking_started"
	RunActivityThinkingFinished RunActivityType = "thinking_finished"
	RunActivityToolStarted      RunActivityType = "tool_started"
	RunActivityToolFinished     RunActivityType = "tool_finished"
)

// RunActivity describes a state transition for display to non-interactive
// clients. Reasoning text and tool result contents are intentionally omitted.
type RunActivity struct {
	SessionID        string
	RunID            string
	MessageID        string
	Type             RunActivityType
	ThinkingDuration time.Duration
	ToolCall         message.ToolCall
	ToolResult       message.ToolResult
	Cancelled        bool
}
