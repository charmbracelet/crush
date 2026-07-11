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

func TestPrintedToolEnvelope(t *testing.T) {
	t.Parallel()
	require.True(t, printedToolEnvelope("I'll run this:\n```json\n{\"tool_name\":\"bash\",\"command\":\"pwd\"}\n```"))
	require.True(t, printedToolEnvelope(`{"tool_name":"view","arguments":{"path":"README.md"}}`))
	require.False(t, printedToolEnvelope(`{"command":"pwd"}`))
}

func TestRecoveryPromptSkipsAcknowledgement(t *testing.T) {
	t.Parallel()
	got := recoveryPrompt("ok", "Choose another approach.")
	require.Contains(t, got, "the unresolved user goal described in the session context")
	require.NotContains(t, got, "Continue ok")
}

func TestAnnouncedPendingAction(t *testing.T) {
	t.Parallel()

	require.True(t, announcedPendingAction("The file is minified, so I'll insert the MCP section next:"))
	require.True(t, announcedPendingAction("I need to inspect the active configuration."))
	require.False(t, announcedPendingAction("Configured and verified all MCP clients."))
}
