package chat

import (
	"testing"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// TestAssistantItemCopyRestoresLinks guards the end-to-end selection
// copy path: glamour emits link targets as OSC 8 hyperlinks, and the
// full item render (borders, padding, and prefixing included) must
// carry them into the highlight-copy extraction so a copied link comes
// out as raw markdown rather than the rendered "text url" form.
func TestAssistantItemCopyRestoresLinks(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	msg := &message.Message{
		ID:   "copy-links",
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: "See [the docs](https://example.com/x) for details."},
			message.Finish{Reason: message.FinishReasonEndTurn, Time: 1},
		},
	}
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)

	const width = 80
	rendered := item.RawRender(width)
	copied := list.HighlightContent(rendered, uv.Rect(0, 0, width, lipgloss.Height(rendered)), 0, 0, -1, -1)
	require.Contains(t, copied, "[the docs](https://example.com/x)",
		"the full item render must keep the OSC 8 link target for the copy, got:\n%s", copied)
}

// TestAssistantItemCopyRestoresEmphasis guards that the emphasis
// sentinel cells survive the full item render path and that a copy of
// a rendered assistant message restores the raw markers.
func TestAssistantItemCopyRestoresEmphasis(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	msg := &message.Message{
		ID:   "copy-emphasis",
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: "Some *emph* and **strong** text."},
			message.Finish{Reason: message.FinishReasonEndTurn, Time: 1},
		},
	}
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)

	const width = 80
	rendered := item.RawRender(width)
	copied := list.HighlightContent(rendered, uv.Rect(0, 0, width, lipgloss.Height(rendered)), 0, 0, -1, -1)
	require.Contains(t, copied, "Some *emph* and **strong** text.",
		"the full item render must keep the emphasis sentinels for the copy, got:\n%s", copied)
}
