package model

import (
	"encoding/json"
	"testing"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func replayDispatchItem(t *testing.T, sty *styles.Styles, messageID, callID, label string, refused bool) *chat.AgentToolMessageItem {
	t.Helper()
	input, err := json.Marshal(agent.AgentDispatchParams{Prompt: "go", Label: label})
	require.NoError(t, err)
	call := message.ToolCall{ID: callID, Name: agent.AgentDispatchToolName, Input: string(input), Finished: true}
	result := &message.ToolResult{ToolCallID: callID, Name: agent.AgentDispatchToolName, Content: "started", IsError: refused}
	item, ok := chat.NewToolMessageItem(sty, messageID, call, result, false, "").(*chat.AgentToolMessageItem)
	require.True(t, ok)
	return item
}

// TestReplayDispatchReportsSettlesDeadSubAgents covers reloading a session
// whose sub-agents are not all running. Only the latest, unreported, live
// dispatch under a label may keep spinning; one cut off when Crush exited, an
// older dispatch under a reused label, and a refused dispatch all settle.
func TestReplayDispatchReportsSettlesDeadSubAgents(t *testing.T) {
	t.Parallel()
	sty := styles.CharmtonePantera()

	live := replayDispatchItem(t, &sty, "m1", "c1", "live", false)
	dead := replayDispatchItem(t, &sty, "m1", "c2", "dead", false)
	old := replayDispatchItem(t, &sty, "m1", "c3", "reused", false)
	newer := replayDispatchItem(t, &sty, "m2", "c4", "reused", false)
	refused := replayDispatchItem(t, &sty, "m2", "c5", "live", true)

	msgs := []*message.Message{
		{ID: "m1", Role: message.Assistant, Parts: []message.ContentPart{live.ToolCall(), dead.ToolCall(), old.ToolCall()}},
		{ID: "m2", Role: message.Assistant, Parts: []message.ContentPart{newer.ToolCall(), refused.ToolCall()}},
		{ID: "t2", Role: message.Tool, Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "c5", Name: agent.AgentDispatchToolName, IsError: true},
		}},
	}
	busy := map[string]bool{"c1": true, "c3": true, "c4": true}

	items := []chat.MessageItem{live, dead, old, newer, refused}
	replayDispatchReports(items, msgs, func(_, callID string) bool { return busy[callID] })

	running := func(item *chat.AgentToolMessageItem) bool {
		status, ok := item.DispatchStatus()
		require.True(t, ok)
		return status.Running
	}
	require.True(t, running(live))
	require.False(t, running(dead), "a dispatch cut off by a restart must stop spinning")
	require.False(t, running(old), "only the latest dispatch under a label can be running")
	require.True(t, running(newer))
	require.False(t, running(refused), "a refused dispatch never started")
}
