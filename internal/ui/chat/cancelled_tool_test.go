package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

// A tool caught by a cancelled turn holds an interruption dressed as an
// error, so it rendered a red ERROR banner for something the user did on
// purpose.
func TestCancelledTurnRendersToolAsCancelled(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	item := NewToolMessageItem(&sty, "m1",
		message.ToolCall{ID: "c1", Name: "mcp__prickly__find", Input: `{}`, Finished: true},
		&message.ToolResult{ToolCallID: "c1", Content: "context canceled", IsError: true},
		true, // the turn was cancelled
		"/tmp",
	)

	out := plainText(item.Render(120))
	require.Contains(t, out, "Canceled")
	require.NotContains(t, out, "context canceled", "the runtime's wording must not reach the user")
}

// Without a cancellation, a genuine tool error still reads as an error.
func TestToolErrorStillRendersAsError(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	item := NewToolMessageItem(&sty, "m1",
		message.ToolCall{ID: "c1", Name: "mcp__prickly__find", Input: `{}`, Finished: true},
		&message.ToolResult{ToolCallID: "c1", Content: "no such element", IsError: true},
		false,
		"/tmp",
	)

	require.Contains(t, plainText(item.Render(120)), "no such element")
}

// plainText drops ANSI styling so assertions read the visible text.
func plainText(s string) string {
	var b strings.Builder
	var esc bool
	for _, r := range s {
		switch {
		case r == 0x1b:
			esc = true
		case esc && r == 'm':
			esc = false
		case !esc:
			b.WriteRune(r)
		}
	}
	return b.String()
}
