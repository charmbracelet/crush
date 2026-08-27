package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

// interruptedTurn is the assistant message a run leaves behind when it
// dies mid-turn: no finish part and a tool call with no result.
func interruptedTurn() *message.Message {
	return &message.Message{
		ID:        "assistant-1",
		SessionID: "s1",
		Role:      message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{
				ID:       "call-1",
				Name:     "bash",
				Input:    `{"command":"echo hi"}`,
				Finished: true,
			},
		},
	}
}

// TestFrozenTranscriptSpinsForever documents the state the load-time
// repair exists to remove: with no finish part and no tool result the
// chat reports work in progress on a session that has none.
func TestFrozenTranscriptSpinsForever(t *testing.T) {
	sty := styles.CharmtonePantera()
	msg := interruptedTurn()
	items := ExtractMessageItems(&sty, msg, nil, "/tmp")
	require.Len(t, items, 1, "a tool-only turn renders just the tool row")
	require.False(t, items[0].Finished(), "the tool row never reaches a terminal state")
	require.Contains(t, items[0].Render(80), "Waiting for tool response")
}

// TestRepairedTranscriptRendersInterrupted is the render half of the
// load-time repair: once the stored transcript carries the finish part
// and the error tool result that agent.RepairInterruptedSession writes,
// nothing on screen claims the session is still working.
func TestRepairedTranscriptRendersInterrupted(t *testing.T) {
	sty := styles.CharmtonePantera()
	msg := interruptedTurn()
	// What the repair persists.
	msg.AddFinish(message.FinishReasonError, "Interrupted",
		"The run ended before this response finished.")
	results := map[string]message.ToolResult{
		"call-1": {
			ToolCallID: "call-1",
			Name:       "bash",
			Content:    "No result was recorded for this tool call.",
			IsError:    true,
		},
	}

	items := ExtractMessageItems(&sty, msg, results, "/tmp")
	require.Len(t, items, 2, "the interrupted banner renders alongside the tool row")

	var out strings.Builder
	for _, item := range items {
		require.True(t, item.Finished(), "no item may still report work in progress")
		out.WriteString(item.Render(80))
		out.WriteString("\n")
	}
	rendered := out.String()
	require.Contains(t, rendered, "Interrupted", "the user must see why the turn ended")
	require.NotContains(t, rendered, "Waiting for tool response")
}
