package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// TestAssistantMessageItemHideThinking guards the display-only reasoning
// toggle: hiding the thinking section must drop the reasoning text and the
// clickable box while leaving the answer in place.
func TestAssistantMessageItemHideThinking(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	msg := thinkingMessage("hidden-thinking", "secret reasoning", "visible answer")
	item := newAssistantMessageItem(&sty, msg, true)

	// Unique odd widths keep these tests off any width shared with the
	// other parallel tests, whose glamour renderer is memoized per width.
	out := ansi.Strip(item.RawRender(913))
	require.NotContains(t, out, "secret reasoning")
	require.Contains(t, out, "visible answer")
	require.Zero(t, item.thinkingBoxHeight,
		"a hidden thinking box must not stay clickable")
	require.False(t, item.ToggleExpanded(),
		"a hidden thinking section must not be expandable")
}

// TestAssistantMessageItemHideThinkingOnlyMessage guards that a message
// carrying nothing but reasoning hides the reasoning while the working
// spinner of the still-thinking turn remains.
func TestAssistantMessageItemHideThinkingOnlyMessage(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	msg := thinkingMessage("only-thinking", "secret reasoning", "")
	item := newAssistantMessageItem(&sty, msg, true)

	out := ansi.Strip(item.RawRender(915))
	require.NotContains(t, out, "secret reasoning")
	require.NotEmpty(t, strings.TrimSpace(out),
		"the turn's working spinner must still render")
}

// TestAssistantMessageItemShowThinkingByDefault guards that the zero value
// keeps the previous behaviour, so only callers that ask for it hide the
// reasoning section.
func TestAssistantMessageItemShowThinkingByDefault(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	msg := thinkingMessage("visible-thinking", "secret reasoning", "visible answer")
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)

	out := ansi.Strip(item.RawRender(917))
	require.Contains(t, out, "secret reasoning")
	require.Contains(t, out, "visible answer")
}

// TestAssistantMessageItemSetHideThinking guards the in-place toggle the UI
// uses, including toggling back on and keeping streaming updates hidden.
func TestAssistantMessageItemSetHideThinking(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	msg := thinkingMessage("toggled-thinking", "secret reasoning", "visible answer")
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)

	item.SetHideThinking(true)
	out := ansi.Strip(item.RawRender(919))
	require.NotContains(t, out, "secret reasoning")

	item.SetMessage(thinkingMessage("toggled-thinking", "fresh reasoning", "visible answer"))
	out = ansi.Strip(item.RawRender(919))
	require.NotContains(t, out, "fresh reasoning")
	require.Contains(t, out, "visible answer")

	item.SetHideThinking(false)
	out = ansi.Strip(item.RawRender(919))
	require.Contains(t, out, "fresh reasoning")
	require.Contains(t, out, "visible answer")
}

// TestExtractMessageItemsHideThinking guards that the flag reaches the
// assistant item through the extraction path the UI actually uses.
func TestExtractMessageItemsHideThinking(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	msg := thinkingMessage("extracted-thinking", "secret reasoning", "visible answer")
	items := ExtractMessageItems(&sty, msg, nil, "", true)
	require.Len(t, items, 1)

	item, ok := items[0].(*AssistantMessageItem)
	require.True(t, ok)
	out := ansi.Strip(item.RawRender(921))
	require.NotContains(t, out, "secret reasoning")
	require.Contains(t, out, "visible answer")
}
