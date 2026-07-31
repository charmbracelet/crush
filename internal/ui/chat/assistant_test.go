package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// TestAssistantMessageItemExpandable guards the Expandable contract on
// AssistantMessageItem along the keyboard-driven expand path. The earlier
// implementation returned no value, which meant the type silently did
// not satisfy chat.Expandable and the keyboard-driven expand path in
// model/chat.go skipped thinking blocks.
//
// We exercise the contract through the bare Expandable interface (the
// same dispatch site model.Chat.ToggleExpandedSelectedItem uses), which
// proves both that AssistantMessageItem still satisfies the interface
// and that the bool return reports the right semantic state at every
// point in the cycle.
func TestAssistantMessageItemExpandable(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	// Short thinking: under the tail-window cap, so the cycle is
	// collapsed -> full -> collapsed (tail-window is skipped).
	msg := thinkingMessage("m1", "step one\nstep two\nstep three", "")
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)

	exp, ok := any(item).(Expandable)
	require.True(t, ok, "AssistantMessageItem must satisfy Expandable")

	require.Equal(t, thinkingCollapsed, item.thinkingViewMode,
		"new items must start in the collapsed view-mode")
	require.True(t, exp.ToggleExpanded(),
		"first toggle of a non-empty thinking block must report expanded")
	require.Equal(t, thinkingFullExpanded, item.thinkingViewMode,
		"short blocks must skip tail-window and land in full expansion")
	require.False(t, exp.ToggleExpanded(),
		"second toggle must report collapsed (cycle closed)")
	require.Equal(t, thinkingCollapsed, item.thinkingViewMode)
}

// TestAssistantMessageItemExpandableEmptyThinkingNoOp guards the B2
// fix: a message with no thinking text must treat ToggleExpanded as a
// no-op. Mutating the view mode in that case would thrash the
// thinking-section cache key for no visible benefit and would surprise
// the caller (model.Chat.ToggleExpandedSelectedItem would treat a
// "now collapsed" return as a real state change and re-scroll on it).
func TestAssistantMessageItemExpandableEmptyThinkingNoOp(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	msg := &message.Message{ID: "m1-empty", Role: message.Assistant}
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)

	exp, ok := any(item).(Expandable)
	require.True(t, ok, "AssistantMessageItem must satisfy Expandable")

	require.Equal(t, thinkingCollapsed, item.thinkingViewMode)
	require.False(t, exp.ToggleExpanded(),
		"empty thinking must report current (collapsed) state without flipping")
	require.Equal(t, thinkingCollapsed, item.thinkingViewMode,
		"empty-thinking toggle must not mutate thinkingViewMode")

	// Whitespace-only thinking is still effectively empty.
	item.message.Parts = []message.ContentPart{
		message.ReasoningContent{Thinking: "  \n\n\t  ", StartedAt: testStartedAt},
	}
	require.False(t, exp.ToggleExpanded())
	require.Equal(t, thinkingCollapsed, item.thinkingViewMode)
}

// TestAssistantMessageItemTailWindowBoundary guards the B1 fix: the
// tail-window heuristic must compare logical line counts (1 +
// newline count) against the cap, not raw newline counts. A source
// whose logical line count exactly equals the cap must NOT trip the
// tail-window step (full render still fits cleanly under the cap),
// while one logical line over the cap must trip it.
func TestAssistantMessageItemTailWindowBoundary(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()

	atCap := buildLines(maxExpandedThinkingTailLines)
	overCap := buildLines(maxExpandedThinkingTailLines + 1)

	atItem := NewAssistantMessageItem(&sty, thinkingMessage("at-cap", atCap, "")).(*AssistantMessageItem)
	require.False(t, atItem.tailWindowWouldTruncate(),
		"a source with exactly N logical lines must not trip the tail-window step")

	overItem := NewAssistantMessageItem(&sty, thinkingMessage("over-cap", overCap, "")).(*AssistantMessageItem)
	require.True(t, overItem.tailWindowWouldTruncate(),
		"a source with N+1 logical lines must trip the tail-window step")
}

// buildLines returns a string of n logical lines (n-1 newlines). Each
// line is a unique short token so callers can distinguish head from
// tail in rendered output if they need to.
func buildLines(n int) string {
	if n <= 0 {
		return ""
	}
	var b []byte
	for i := 1; i <= n; i++ {
		if i > 1 {
			b = append(b, '\n')
		}
		b = append(b, 'l', 'n')
		b = append(b, []byte(itoa(i))...)
	}
	return string(b)
}

// TestAssistantMessageItemHandleMouseClick ensures HandleMouseClick does not
// toggle expansion on its own. The generic Expandable path in
// model/chat.go does the toggle; doing it here too would double-toggle and
// net to no change.
func TestAssistantMessageItemHandleMouseClick(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	msg := &message.Message{ID: "m2", Role: message.Assistant}
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)
	item.thinkingBoxHeight = 5

	// Click inside the thinking box signals handled but must not mutate
	// the view-mode state.
	require.True(t, item.HandleMouseClick(ansi.MouseLeft, 0, 2))
	require.Equal(t, thinkingCollapsed, item.thinkingViewMode,
		"HandleMouseClick must not toggle expansion on its own")

	// Click outside the thinking box is ignored entirely.
	require.False(t, item.HandleMouseClick(ansi.MouseLeft, 0, 10))
	require.Equal(t, thinkingCollapsed, item.thinkingViewMode)

	// Non-left button is ignored.
	require.False(t, item.HandleMouseClick(ansi.MouseRight, 0, 2))
	require.Equal(t, thinkingCollapsed, item.thinkingViewMode)
}

// finishedTextMessage builds an assistant message with the given Markdown
// text and an end_turn finish part, so the click-to-copy footer renders.
func finishedTextMessage(id, text string) *message.Message {
	return &message.Message{
		ID:   id,
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: text},
			message.Finish{Reason: message.FinishReasonEndTurn, Time: testFinishTime},
		},
	}
}

// TestAssistantMessageItemCopyFooterRendered guards the render side of the
// click-to-copy footer: a finished message with content must render the ⎘
// footer on its own row at the right edge, and the recorded click geometry
// must agree with the rendered output.
func TestAssistantMessageItemCopyFooterRendered(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	msg := finishedTextMessage("copy-1", "hello **world**")
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)

	const width = 40
	rendered := item.RawRender(width)
	lines := strings.Split(rendered, "\n")

	require.GreaterOrEqual(t, item.copyIconRow, 0, "finished message with content must render the copy footer")
	require.Less(t, item.copyIconRow, len(lines), "recorded footer row must be within the rendered output")
	// The blank separator line precedes the footer row.
	require.Empty(t, strings.TrimSpace(lines[item.copyIconRow-1]),
		"the line above the footer must be the blank separator")

	footerLine := lines[item.copyIconRow]
	require.Contains(t, footerLine, "⎘", "footer line must contain the copy glyph")
	// The glyph must be right-aligned inside the capped content width:
	// strip ANSI styling before measuring its cell offset within the line.
	plain := ansi.Strip(footerLine)
	require.Equal(t, item.copyIconColStart, len([]rune(strings.Split(plain, "⎘")[0])),
		"icon must sit at the recorded start column")
	require.Equal(t, item.copyIconColStart+1, item.copyIconColEnd,
		"the single-cell glyph must span exactly one column")
}

// TestAssistantMessageItemCopyFooterSuppressed guards the states where the
// footer must not render: a still-streaming message and an empty finished
// message.
func TestAssistantMessageItemCopyFooterSuppressed(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()

	// Streaming: no Finish part yet, so IsFinished is false.
	streaming := NewAssistantMessageItem(&sty, thinkingMessage("copy-2", "", "partial")).(*AssistantMessageItem)
	streaming.RawRender(40)
	require.Equal(t, -1, streaming.copyIconRow,
		"streaming message must not render the copy footer")

	// Finished but empty content: nothing worth copying.
	empty := NewAssistantMessageItem(&sty, &message.Message{
		ID:   "copy-3",
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.Finish{Reason: message.FinishReasonEndTurn, Time: testFinishTime},
		},
	}).(*AssistantMessageItem)
	empty.RawRender(40)
	require.Equal(t, -1, empty.copyIconRow,
		"empty finished message must not render the copy footer")
}

// TestAssistantMessageItemCopyIconClick guards the click contract: a click
// on the footer glyph must be handled, return a non-nil command (the
// clipboard copy), and leave the view-mode untouched so the generic
// expansion path cannot fire. Clicks elsewhere on the footer row or with
// the wrong button must not copy.
func TestAssistantMessageItemCopyIconClick(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	msg := finishedTextMessage("copy-4", "raw **markdown** body")
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)
	item.RawRender(40)
	require.GreaterOrEqual(t, item.copyIconRow, 0)

	// Click on the glyph: x is chat-relative, so the content column is
	// offset by MessageLeftPaddingTotal.
	handled, cmd := item.HandleMouseClickCmd(ansi.MouseLeft,
		item.copyIconColStart+MessageLeftPaddingTotal, item.copyIconRow)
	require.True(t, handled, "click on the copy glyph must be handled")
	require.NotNil(t, cmd, "click on the copy glyph must produce the copy command")
	require.Equal(t, thinkingCollapsed, item.thinkingViewMode,
		"a copy click must not disturb the expansion state machine")

	// Click to the left of the glyph on the same row is not a copy.
	handled, cmd = item.HandleMouseClickCmd(ansi.MouseLeft, 0, item.copyIconRow)
	require.False(t, handled, "click outside the glyph span must not copy")
	require.Nil(t, cmd)

	// Right button on the glyph is ignored.
	handled, cmd = item.HandleMouseClickCmd(ansi.MouseRight,
		item.copyIconColStart+MessageLeftPaddingTotal, item.copyIconRow)
	require.False(t, handled)
	require.Nil(t, cmd)

	// The plain MouseClickable path (defensive: chat falls back to it)
	// must agree the glyph click is handled without producing a toggle.
	require.True(t, item.HandleMouseClick(ansi.MouseLeft,
		item.copyIconColStart+MessageLeftPaddingTotal, item.copyIconRow))
	require.Equal(t, thinkingCollapsed, item.thinkingViewMode)
}

// TestAssistantMessageItemThinkingClickStillExpands guards against the new
// commandable path breaking the old expansion contract: a click on the
// thinking box reports handled with a nil command so the generic
// Expandable toggle in model/chat.go still fires.
func TestAssistantMessageItemThinkingClickStillExpands(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	msg := &message.Message{ID: "copy-5", Role: message.Assistant}
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)
	item.thinkingBoxHeight = 5

	handled, cmd := item.HandleMouseClickCmd(ansi.MouseLeft, 0, 2)
	require.True(t, handled, "thinking-box click must still be handled")
	require.Nil(t, cmd, "thinking-box click must not produce a command (expansion proceeds)")
}
