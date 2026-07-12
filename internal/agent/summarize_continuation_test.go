package agent

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/fantasy"
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

func TestRecoveryContextDoesNotNestPriorRecoveryText(t *testing.T) {
	t.Parallel()
	call := SessionAgentCall{
		Prompt:           "<recovery_state>old wrapper</recovery_state>",
		originalIntent:   "enable github and gh_grep MCP",
		recoveryGuidance: "run mcp_refresh",
	}

	got := recoveryContext(originalIntent(call), call.recoveryGuidance)
	require.Contains(t, got, "<original_user_intent>enable github and gh_grep MCP</original_user_intent>")
	require.NotContains(t, got, "old wrapper")
	require.Equal(t, 1, strings.Count(got, "<recovery_state>"))
}

func TestRecoveryToolFromReview(t *testing.T) {
	t.Parallel()
	require.Equal(t, "mcp_refresh", recoveryToolFromReview("Auto-review sidecar: verify runtime.\nNext tool: mcp_refresh"))
	require.Empty(t, recoveryToolFromReview("Auto-review sidecar: credentials are missing.\nNext tool: none"))
}

func TestRecoveryMessagesDropsReviewAndBoundsHistory(t *testing.T) {
	t.Parallel()
	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "original"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "Auto-review sidecar: do not persist me"}}},
	}
	for i := range 12 {
		msgs = append(msgs, message.Message{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: fmt.Sprintf("step-%d", i)}}})
	}

	got := recoveryMessages(msgs)
	require.Len(t, got, 10)
	for _, msg := range got {
		require.False(t, isAutoReviewSidecarMessage(msg))
	}
	require.Equal(t, "step-2", got[0].Content().String())

	agent := &sessionAgent{}
	history, _ := agent.preparePrompt([]message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "task"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "Auto-review sidecar: internal diagnosis"}}},
	}, false)
	require.Len(t, history, 2, "system reminder and real user message should remain")
}

func TestAnnouncedPendingAction(t *testing.T) {
	t.Parallel()

	require.True(t, announcedPendingAction("The file is minified, so I'll insert the MCP section next:"))
	require.True(t, announcedPendingAction("I need to inspect the active configuration."))
	require.False(t, announcedPendingAction("Configured and verified all MCP clients."))
}

func TestApplyRequiredFirstToolOnlyAffectsFirstStep(t *testing.T) {
	t.Parallel()

	first := fantasy.PrepareStepResult{}
	applyRequiredFirstTool(&first, "web_search", 0)
	require.NotNil(t, first.ToolChoice)
	require.Equal(t, fantasy.SpecificToolChoice("web_search"), *first.ToolChoice)
	require.Equal(t, []string{"web_search"}, first.ActiveTools)

	later := fantasy.PrepareStepResult{}
	applyRequiredFirstTool(&later, "web_search", 1)
	require.Nil(t, later.ToolChoice)
	require.Empty(t, later.ActiveTools)
}
