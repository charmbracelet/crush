package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestContentHasA2UI(t *testing.T) {
	t.Parallel()
	require.True(t, contentHasA2UI("here you go\n```a2ui\n{}\n```"))
	require.False(t, contentHasA2UI("just a normal ```json\n{}\n``` block"))
	require.False(t, contentHasA2UI("plain prose"))
}

func TestRenderA2UI(t *testing.T) {
	t.Parallel()

	// A valid A2UI card routes through a2tea. The renderers are still stubs,
	// so the element renders as the "[a2tea: card]" placeholder — the point is
	// that crush recognized the document and handed it to a2tea.
	out, ok := renderA2UI(`{"kind":"card","id":"c","title":"Hi"}`, 80)
	require.True(t, ok)
	require.Contains(t, out, "[a2tea: card]")

	// An unknown kind is not valid A2UI: renderA2UI reports false so the caller
	// falls back to plain markdown rather than swallowing the error.
	_, ok = renderA2UI(`{"kind":"table"}`, 80)
	require.False(t, ok)

	// Malformed JSON likewise falls back.
	_, ok = renderA2UI(`{not json`, 80)
	require.False(t, ok)
}

func TestRenderContentWithA2UI(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	item := &AssistantMessageItem{sty: &sty}

	content := "Here is a card:\n\n```a2ui\n{\"kind\":\"card\",\"id\":\"c\",\"title\":\"Hi\"}\n```\n\nAnything else?"
	out := item.renderContentWithA2UI(content, 80)
	// Strip ANSI styling before matching prose — glamour interleaves color
	// codes between words.
	plain := ansi.Strip(out)

	// The a2tea element is stitched in...
	require.Contains(t, out, "[a2tea: card]")
	// ...and the surrounding prose is preserved (before and after the element).
	require.Contains(t, plain, "Here is a card")
	require.Contains(t, plain, "Anything else")
	// The raw JSON is NOT shown as a code block.
	require.NotContains(t, plain, "\"kind\"")
}

func TestRenderContentWithA2UIInvalidBlockFallsBack(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	item := &AssistantMessageItem{sty: &sty}

	// An a2ui block that is not valid A2UI is rendered as its original markdown
	// so nothing is dropped.
	content := "```a2ui\n{\"kind\":\"table\"}\n```"
	out := item.renderContentWithA2UI(content, 80)
	require.NotContains(t, out, "[a2tea:")
	require.True(t, strings.Contains(out, "table") || strings.Contains(out, "kind"))
}
