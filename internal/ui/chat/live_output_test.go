package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// TestBashToolRendersLiveOutputUntilResult verifies that streamed output
// replaces the waiting message and is itself replaced by the final result.
func TestBashToolRendersLiveOutputUntilResult(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	tc := message.ToolCall{ID: "call1", Name: tools.BashToolName, Input: `{"command":"python count.py"}`, Finished: true}
	item := NewBashToolMessageItem(&sty, tc, nil, false, "")
	live := item.(LiveOutputItem)

	require.Contains(t, ansi.Strip(item.Render(100)), "Waiting for tool response")

	require.True(t, live.SetLiveOutput("1\n2"))
	rendered := ansi.Strip(item.Render(100))
	require.Contains(t, rendered, "1")
	require.Contains(t, rendered, "2")
	require.NotContains(t, rendered, "Waiting for tool response")
	require.False(t, live.SetLiveOutput("1\n2"), "unchanged output is a no-op")

	item.SetResult(&message.ToolResult{ToolCallID: "call1", Content: "final-output"})
	rendered = ansi.Strip(item.Render(100))
	require.Contains(t, rendered, "final-output")
	require.False(t, live.SetLiveOutput("late chunk"), "late chunks are ignored after the result")
	require.NotContains(t, ansi.Strip(item.Render(100)), "late chunk")
}

// TestJobOutputToolRendersLiveOutput verifies that a waiting job output call
// shows streamed output under its header.
func TestJobOutputToolRendersLiveOutput(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	tc := message.ToolCall{ID: "call2", Name: tools.JobOutputToolName, Input: `{"shell_id":"001","wait":true}`, Finished: true}
	item := NewJobOutputToolMessageItem(&sty, tc, nil, false, "")
	live := item.(LiveOutputItem)

	require.True(t, live.SetLiveOutput("tick 1\ntick 2"))
	rendered := ansi.Strip(item.Render(100))
	require.Contains(t, rendered, "PID 001")
	require.Contains(t, rendered, "tick 2")
}

// TestLiveOutputShowsTailWhenCollapsed verifies that collapsed live output
// keeps the newest lines rather than the oldest.
func TestLiveOutputShowsTailWhenCollapsed(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	tc := message.ToolCall{ID: "call3", Name: tools.BashToolName, Input: `{"command":"seq 100"}`, Finished: true}
	item := NewBashToolMessageItem(&sty, tc, nil, false, "")

	var lines []string
	for i := range 100 {
		lines = append(lines, "line-"+string(rune('a'+i%26))+strings.Repeat("x", i%3))
	}
	lines[0] = "FIRSTLINE"
	lines[99] = "LASTLINE"
	item.(LiveOutputItem).SetLiveOutput(strings.Join(lines, "\n"))

	rendered := ansi.Strip(item.Render(100))
	require.Contains(t, rendered, "LASTLINE")
	require.NotContains(t, rendered, "FIRSTLINE")
}
