package chat

import (
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
	out, err := renderA2UI(`{"kind":"card","id":"c","title":"Hi"}`, 80)
	require.NoError(t, err)
	require.Contains(t, out, "[a2tea: card]")

	// An unknown kind is not valid A2UI: renderA2UI returns the a2tea error so
	// the caller can show an alert rather than swallowing it.
	_, err = renderA2UI(`{"kind":"table"}`, 80)
	require.Error(t, err)

	// Malformed JSON likewise returns an error.
	_, err = renderA2UI(`{not json`, 80)
	require.Error(t, err)
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

func TestRenderContentWithA2UIInvalidBlockShowsAlert(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	item := &AssistantMessageItem{sty: &sty}

	// An a2ui block that is not valid A2UI is replaced with an alert element
	// that names the failure and echoes the payload — not dropped, not shown as
	// a rendered element.
	content := "```a2ui\n{\"kind\":\"table\"}\n```"
	out := item.renderContentWithA2UI(content, 80)
	plain := ansi.Strip(out)

	require.NotContains(t, plain, "[a2tea:")   // no successful element
	require.Contains(t, plain, "A2UI")         // alert tag
	require.Contains(t, plain, "unknown kind") // the a2tea error reason
	require.Contains(t, plain, "table")        // the echoed payload
}
