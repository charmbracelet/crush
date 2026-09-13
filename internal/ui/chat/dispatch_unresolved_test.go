package chat

import (
	"testing"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func dispatchItem(t *testing.T) *AgentToolMessageItem {
	t.Helper()
	sty := styles.CharmtonePantera()
	item := NewToolMessageItem(&sty, "msg-1", message.ToolCall{
		ID:       "call-1",
		Name:     agent.AgentDispatchToolName,
		Input:    `{"prompt":"look into it","label":"audit"}`,
		Finished: true,
	}, &message.ToolResult{
		ToolCallID: "call-1",
		Content:    "Sub-agent \"audit\" started.",
	}, false, "/tmp")

	dispatch, ok := item.(*AgentToolMessageItem)
	require.True(t, ok)
	require.Equal(t, "audit", dispatch.DispatchLabel())
	return dispatch
}

// A dispatch call is answered the moment its sub-agent starts, so the
// presence of a result says nothing about whether the work is done. Judging
// it by the result alone leaves a finished sub-agent looking like it is
// still running after the session is reopened.
func TestDispatchIsUnresolvedUntilItReports(t *testing.T) {
	t.Parallel()

	dispatch := dispatchItem(t)
	require.True(t, dispatch.Unresolved(), "a launched sub-agent has not answered yet")
	require.True(t, dispatch.Spinning())

	dispatch.MarkDispatchReported()
	require.False(t, dispatch.Unresolved(), "the report settles it")
	require.False(t, dispatch.Spinning())
}

// The plain agent tool answers only when its work is done, so there the
// result really is the signal.
func TestSyncAgentToolResolvesOnResult(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	item := NewToolMessageItem(&sty, "msg-1", message.ToolCall{
		ID: "call-1", Name: agent.AgentToolName, Input: `{"prompt":"go"}`, Finished: true,
	}, &message.ToolResult{ToolCallID: "call-1", Content: "done"}, false, "/tmp")

	require.False(t, item.Unresolved())
}

func TestDispatchCancelledIsResolved(t *testing.T) {
	t.Parallel()

	dispatch := dispatchItem(t)
	dispatch.SetStatus(ToolStatusCanceled)
	require.False(t, dispatch.Unresolved(), "a cancelled dispatch is settled")
}
