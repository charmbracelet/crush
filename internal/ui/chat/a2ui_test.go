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

// --- Issue #6: fenced code blocks should not render as live surfaces ---

func TestContentHasA2UIIgnoresFencedCode(t *testing.T) {
	t.Parallel()
	// A tagged block inside a fenced code block is an example, not live UI.
	fenced := "Here is an example:\n\n```json\n" + a2uiSurface + "\n```\n\nDone."
	require.False(t, contentHasA2UI(fenced))
}

func TestRenderContentWithA2UIFencedCodeNotLiveSurface(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	item := &AssistantMessageItem{sty: &sty}

	content := "Example:\n\n```json\n" + a2uiSurface + "\n```\n\nAnd some text."
	out := item.renderContentWithA2UI(content, 80)
	plain := ansi.Strip(out)

	// The fenced example is rendered as code — the raw tags and JSON are
	// visible as text, NOT extracted as a live surface.
	require.Contains(t, plain, "a2ui-json")
	// No alert — the fenced block is just an example, not a dropped surface.
	require.NotContains(t, plain, "couldn't render")
	// The surrounding text is preserved.
	require.Contains(t, plain, "Example")
	require.Contains(t, plain, "And some text")
}

func TestRenderContentWithA2UIRealSurfaceNextToFencedExample(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	item := &AssistantMessageItem{sty: &sty}

	// A real surface followed by a fenced example: both should render
	// correctly — the real surface as live, the fenced example as code.
	content := "Real: " + a2uiSurface + "\n\nExample:\n\n```json\n" + a2uiSurface + "\n```"
	out := item.renderContentWithA2UI(content, 80)
	plain := ansi.Strip(out)

	require.Contains(t, plain, "Hello from A2UI") // real surface rendered
}

// --- Issue #5: truncated mid-block should show alert ---

func TestContentHasUnclosedA2UI(t *testing.T) {
	t.Parallel()
	require.True(t, contentHasUnclosedA2UI("text <a2ui-json>{\"version\":\"v0"))
	require.False(t, contentHasUnclosedA2UI("text "+a2uiSurface))
	require.False(t, contentHasUnclosedA2UI("plain prose"))
	// Unclosed tag inside a fenced block is not a truncation.
	require.False(t, contentHasUnclosedA2UI("```json\n<a2ui-json>{bad\n```"))
}

func TestRenderTruncatedA2UI(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	item := &AssistantMessageItem{sty: &sty}

	content := "Here is a card:\n\n<a2ui-json>{\"version\":\"v0.9\",\"updateComponents"
	out := item.renderTruncatedA2UI(content, 80)
	plain := ansi.Strip(out)

	// The prose before the truncated tag is preserved.
	require.Contains(t, plain, "Here is a card")
	// The alert is shown.
	require.Contains(t, plain, "couldn't render")
	// The raw partial JSON is NOT shown.
	require.NotContains(t, plain, "updateComponents")
}

// --- Issue #7: bare JSON should not mask dropped tagged block alert ---

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

func TestRenderContentWithA2UIMalformedNotMaskedByBareJSON(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	item := &AssistantMessageItem{sty: &sty}

	// One malformed tagged block + one bare JSON surface. The bare JSON must
	// not offset the count and suppress the alert for the dropped tagged
	// block.
	bareJSON := `{"version":"v0.9","updateComponents":{"surfaceId":"s","components":[` +
		`{"component":"Text","id":"t","text":"bare"}]}}`
	content := "<a2ui-json>{not valid json}</a2ui-json>\n\n" + bareJSON
	out := item.renderContentWithA2UI(content, 80)
	plain := ansi.Strip(out)

	require.Contains(t, plain, "couldn't render") // the malformed tagged block alerted
}

// --- Existing tests ---

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
