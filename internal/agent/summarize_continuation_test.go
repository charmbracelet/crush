package agent

import (
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func TestLastUserIntent(t *testing.T) {
	t.Parallel()
	msgs := []message.Message{
		{Role: message.User, CreatedAt: 1, Parts: []message.ContentPart{message.TextContent{Text: "original task"}}},
		{Role: message.Assistant, CreatedAt: 2, IsSummaryMessage: true, Parts: []message.ContentPart{message.TextContent{Text: "summary"}}},
		{Role: message.User, CreatedAt: 3, Parts: []message.ContentPart{message.TextContent{Text: "ok"}}},
		{Role: message.User, CreatedAt: 4, Parts: []message.ContentPart{message.TextContent{Text: "latest substantive intent"}}},
	}
	require.Equal(t, "latest substantive intent", lastUserIntent(msgs))
}

func TestLastUserIntentSkipsSummaryAndAcknowledgement(t *testing.T) {
	t.Parallel()
	msgs := []message.Message{
		{Role: message.User, IsSummaryMessage: true, Parts: []message.ContentPart{message.TextContent{Text: "summary presented as context"}}},
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "ok"}}},
	}
	require.Empty(t, lastUserIntent(msgs))
}

func TestSummaryContinuationKeepsOnlyContinuationContext(t *testing.T) {
	t.Parallel()

	call := SessionAgentCall{
		Prompt:           "install browser MCP",
		originalIntent:   "install browser MCP",
		TransientContext: "<loaded_skill>internal routing instructions</loaded_skill>",
		Attachments:      []message.Attachment{{FilePath: "old-context.txt"}},
	}
	call = prepareSummaryContinuation(call)

	require.Contains(t, call.TransientContext, "summary_continuation")
	require.NotContains(t, call.TransientContext, "internal routing instructions")
	require.Empty(t, call.Attachments)
	require.Equal(t, "install browser MCP", call.Prompt)
}

func TestSummaryDoesNotDuplicateQueuedContinuation(t *testing.T) {
	t.Parallel()

	require.True(t, shouldCreateSummaryContinuation("install MCP", nil))
	require.False(t, shouldCreateSummaryContinuation("install MCP", []SessionAgentCall{{skipUserMessage: true}}))
}
