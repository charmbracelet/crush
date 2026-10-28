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
		Content:    "Started subagent \"audit\".",
	}, false, "/tmp")

	dispatch, ok := item.(*AgentToolMessageItem)
	require.True(t, ok)
	require.Equal(t, agent.SubagentName("audit"), dispatch.DispatchLabel())
	return dispatch
}

func running(t *testing.T, dispatch *AgentToolMessageItem) bool {
	t.Helper()
	status, ok := dispatch.DispatchStatus()
	require.True(t, ok)
	return status.Running
}

// A dispatch call is answered the moment its subagent starts, so the
// presence of a result says nothing about whether the work is done. Judging
// it by the result alone leaves a finished subagent looking like it is
// still running after the session is reopened.
func TestDispatchRunsUntilItReports(t *testing.T) {
	t.Parallel()

	dispatch := dispatchItem(t)
	require.True(t, running(t, dispatch), "a launched subagent has not answered yet")
	require.True(t, dispatch.Spinning())

	dispatch.MarkDispatchReported()
	require.False(t, running(t, dispatch), "the report settles it")
	require.False(t, dispatch.Spinning())
}

// The plain agent tool is not a dispatch, so it carries no dispatch status.
func TestSyncAgentToolHasNoDispatchStatus(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	item := NewToolMessageItem(&sty, "msg-1", message.ToolCall{
		ID: "call-1", Name: agent.AgentToolName, Input: `{"prompt":"go"}`, Finished: true,
	}, &message.ToolResult{ToolCallID: "call-1", Content: "done"}, false, "/tmp")

	_, ok := item.(*AgentToolMessageItem).DispatchStatus()
	require.False(t, ok)
}

func TestDispatchCancelledIsSettled(t *testing.T) {
	t.Parallel()

	dispatch := dispatchItem(t)
	dispatch.SetStatus(ToolStatusCanceled)
	require.False(t, running(t, dispatch), "a cancelled dispatch is settled")
}
