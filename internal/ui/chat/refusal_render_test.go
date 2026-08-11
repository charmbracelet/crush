package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

// TestRefusalFinishRendersBanner asserts that a content-filter finish
// (Anthropic stop_reason=refusal mapped through fantasy) paints a
// visible REFUSED banner rather than an empty assistant turn. The
// agent persists only the reason, so the banner text comes from the
// TUI's canonical refusal copy.
func TestRefusalFinishRendersBanner(t *testing.T) {
	sty := styles.CharmtonePantera()
	msg := &message.Message{
		ID:   "refusal-1",
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.Finish{
				Reason: message.FinishReasonContentFilter,
				Time:   testFinishTime,
			},
		},
	}

	require.True(t, ShouldRenderAssistantMessage(msg),
		"empty-body refusal turns must still render so the banner is visible")

	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)
	out := item.Render(80)
	require.Contains(t, out, refusalTagLabel, "banner tag")
	require.Contains(t, out, refusalTitle, "title")
	require.Contains(t, strings.ToLower(out), "safety classifier", "details")
}

// TestRefusalPersistedCopyWins asserts that when a finish part does
// carry its own message/details (e.g. a future provider that supplies
// them), the persisted copy takes precedence over the TUI defaults.
func TestRefusalPersistedCopyWins(t *testing.T) {
	sty := styles.CharmtonePantera()
	msg := &message.Message{
		ID:   "refusal-2",
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.Finish{
				Reason:  message.FinishReasonContentFilter,
				Message: "Custom refusal title",
				Details: "Custom refusal details.",
				Time:    testFinishTime,
			},
		},
	}
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)
	out := item.Render(80)
	require.Contains(t, out, refusalTagLabel)
	require.Contains(t, out, "Custom refusal title")
	require.Contains(t, out, "Custom refusal details.")
}

// TestErrorEmptyDetailsNoTrailingBlock locks in the behavior that an
// error finish with no details renders only the title line, without an
// empty styled details block or trailing blank lines.
func TestErrorEmptyDetailsNoTrailingBlock(t *testing.T) {
	sty := styles.CharmtonePantera()
	msg := &message.Message{
		ID:   "error-1",
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.Finish{
				Reason:  message.FinishReasonError,
				Message: "boom",
				Time:    testFinishTime,
			},
		},
	}
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)
	out := item.Render(80)
	require.Contains(t, out, "ERROR")
	require.Contains(t, out, "boom")
	require.False(t, strings.HasSuffix(out, "\n"),
		"empty-details error must not render a trailing details block")
}
