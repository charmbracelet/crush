package msg

import (
	"time"

	"github.com/charmbracelet/crush/internal/message"
	tea "charm.land/bubbletea/v2"
)

// TUI messages for tool timer updates

// ToolTimerUpdateMsg updates a tool call with current timer state
type ToolTimerUpdateMsg struct {
	ToolCall message.ToolCall
}

// ToolTimerCmd creates a command that updates tool timers every second
func ToolTimerCmd(toolCall message.ToolCall) tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return ToolTimerUpdateMsg{
			ToolCall: toolCall,
		}
	})
}