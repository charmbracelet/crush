package chat

import (
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// renderTool renders a finished bash call with the given result as plain text.
func renderTool(t *testing.T, result message.ToolResult, canceled bool) string {
	t.Helper()
	sty := styles.CharmtonePantera()
	result.ToolCallID = "c1"
	item := NewToolMessageItem(&sty, "m1",
		message.ToolCall{ID: "c1", Name: "bash", Input: `{"command":"ls"}`, Finished: true},
		&result, canceled, "/tmp",
	)
	return ansi.Strip(item.Render(120))
}

// An error result on a cancelled turn is the interruption, not a tool failure;
// "context canceled" is what sessions saved before the agent relabelled it hold.
func TestCancelledTurnRendersToolErrorAsCancelled(t *testing.T) {
	t.Parallel()

	for _, content := range []string{"tool call was interrupted and did not produce a result", "context canceled"} {
		out := renderTool(t, message.ToolResult{Content: content, IsError: true}, true)
		require.Contains(t, out, "Canceled")
		require.NotContains(t, out, content)
	}
}

// A tool that finished before the turn was cancelled keeps its output.
func TestCancelledTurnKeepsSuccessfulToolOutput(t *testing.T) {
	t.Parallel()

	out := renderTool(t, message.ToolResult{Content: "main.go"}, true)
	require.Contains(t, out, "main.go")
	require.NotContains(t, out, "Canceled")
}

// Without a cancellation, a genuine tool error still reads as an error.
func TestToolErrorStillRendersAsError(t *testing.T) {
	t.Parallel()

	require.Contains(t, renderTool(t, message.ToolResult{Content: "no such file", IsError: true}, false), "no such file")
}
