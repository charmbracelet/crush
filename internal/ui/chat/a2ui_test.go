package chat

import (
	"testing"

	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// a2uiSurface is an <a2ui-json> block: a card wrapping a single text component.
const a2uiSurface = `<a2ui-json>{"version":"v0.9","updateComponents":{"surfaceId":"s","components":[` +
	`{"component":"Card","id":"root","child":"t"},` +
	`{"component":"Text","id":"t","text":"Hello from A2UI"}` +
	`]}}</a2ui-json>`

func TestContentHasA2UI(t *testing.T) {
	t.Parallel()
	require.True(t, contentHasA2UI("here you go\n"+a2uiSurface))
	require.False(t, contentHasA2UI("just a normal ```json\n{}\n``` block"))
	require.False(t, contentHasA2UI("plain prose"))
}

func TestRenderContentWithA2UI(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	item := &AssistantMessageItem{sty: &sty}

	content := "Here is a card:\n\n" + a2uiSurface + "\n\nAnything else?"
	out := item.renderContentWithA2UI(content, 80)
	plain := ansi.Strip(out)

	// The A2UI surface renders (text pulled from the card's Text component)...
	require.Contains(t, plain, "Hello from A2UI")
	// ...and the surrounding prose is preserved on both sides.
	require.Contains(t, plain, "Here is a card")
	require.Contains(t, plain, "Anything else")
	// The raw A2UI JSON / tags are NOT shown verbatim.
	require.NotContains(t, plain, "a2ui-json")
	require.NotContains(t, plain, "updateComponents")
	// No alert when the surface rendered fine.
	require.NotContains(t, plain, "couldn't render")
}

func TestRenderContentWithA2UIMalformedShowsAlert(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	item := &AssistantMessageItem{sty: &sty}

	// Malformed JSON inside the tags: a2uistream drops it (no messages), so
	// crush must alert rather than silently losing the block.
	content := "Look: <a2ui-json>{not valid json}</a2ui-json>"
	out := item.renderContentWithA2UI(content, 80)
	plain := ansi.Strip(out)

	require.Contains(t, plain, "A2UI")
	require.Contains(t, plain, "couldn't render")
	// The surrounding prose is still there.
	require.Contains(t, plain, "Look")
}

func TestRenderContentWithA2UIMixedGoodAndBadAlerts(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	item := &AssistantMessageItem{sty: &sty}

	// One valid surface plus one malformed block: the good one renders AND the
	// dropped one is still surfaced via an alert (not silently lost).
	content := "ok: " + a2uiSurface + " bad: <a2ui-json>{nope}</a2ui-json>"
	out := item.renderContentWithA2UI(content, 80)
	plain := ansi.Strip(out)

	require.Contains(t, plain, "Hello from A2UI") // the good surface rendered
	require.Contains(t, plain, "couldn't render") // the bad block alerted
}
