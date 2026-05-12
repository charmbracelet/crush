package chat

import (
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

// renderCountingItem wraps a real chat item and counts Render calls
// to expose the list-level cache behaviour to tests. The wrapper
// forwards the list.Item methods exercised by this test — Render,
// Version, Finished — plus the list.Highlightable surface
// (SetHighlight / Highlight) used by the callback-driven scenario.
// Focus is not exercised here, so list.Focusable is not forwarded;
// add SetFocused/IsFocused if a future test needs to drive focus
// through the wrapper.
type renderCountingItem struct {
	inner       MessageItem
	renderHits  int
	highlightCb func(start [4]int)
}

func newRenderCountingItem(inner MessageItem) *renderCountingItem {
	return &renderCountingItem{inner: inner}
}

func (r *renderCountingItem) Render(width int) string {
	r.renderHits++
	return r.inner.Render(width)
}

func (r *renderCountingItem) Version() uint64 {
	return r.inner.(versionedItem).Version()
}

func (r *renderCountingItem) Finished() bool {
	return r.inner.Finished()
}

// SetHighlight forwards to the embedded item; the underlying
// highlightableMessageItem dedupes equivalent ranges and bumps the
// shared version on observable change.
func (r *renderCountingItem) SetHighlight(startLine, startCol, endLine, endCol int) {
	if h, ok := r.inner.(list.Highlightable); ok {
		h.SetHighlight(startLine, startCol, endLine, endCol)
		if r.highlightCb != nil {
			r.highlightCb([4]int{startLine, startCol, endLine, endCol})
		}
	}
}

func (r *renderCountingItem) Highlight() (int, int, int, int) {
	if h, ok := r.inner.(list.Highlightable); ok {
		return h.Highlight()
	}
	return -1, -1, -1, -1
}

// TestList_CallbackDrivenHighlightUnfreezeAndReFreeze covers F6
// §4.5.1 along the live applyHighlightRange path. Instead of
// driving BeginSelectionDrag directly, the test registers a render
// callback that mutates the chat items' highlight ranges (just like
// Chat.applyHighlightRange does in production) and verifies the
// resulting cache behaviour:
//
//   - Items inside the active range pick up a SetHighlight call,
//     their version bumps, the F6 cache invalidates, and the list
//     re-renders them on the next draw. The post-render entry is
//     frozen again because the items are Finished() — but their
//     stored output now reflects the highlight.
//   - Subsequent draws while the range is unchanged are cache hits:
//     the callback's SetHighlight call dedupes (same range), the
//     version is stable, and the list serves the previous output
//     verbatim without calling Render.
//   - When the range moves OFF an item, the callback clears the
//     highlight, the version bumps, and the item re-renders. After
//     that single re-render the entry re-freezes; further draws are
//     cache hits.
func TestList_CallbackDrivenHighlightUnfreezeAndReFreeze(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()

	// Build three finished assistant messages so all three are
	// candidates for freezing. Real items (per Round 2 spec) — the
	// surrounding renderCountingItem wrapper just lets the test see
	// per-item Render calls.
	mk := func(id, body string) *renderCountingItem {
		msg := &message.Message{
			ID:   id,
			Role: message.Assistant,
			Parts: []message.ContentPart{
				message.ReasoningContent{
					Thinking:   "thinking",
					StartedAt:  testStartedAt,
					FinishedAt: testFinishedAt,
				},
				message.TextContent{Text: body},
				message.Finish{Reason: message.FinishReasonEndTurn, Time: testFinishTime},
			},
		}
		inner := NewAssistantMessageItem(&sty, msg)
		require.True(t, inner.Finished(), "test fixture must be Finished()")
		return newRenderCountingItem(inner)
	}

	a := mk("a", "alpha")
	b := mk("b", "bravo")
	c := mk("c", "charlie")

	l := list.NewList(a, b, c)
	l.SetSize(80, 30)

	// activeRange holds the inclusive [start, end] item indexes the
	// callback should highlight. -1 means no active selection.
	activeRange := [2]int{-1, -1}

	cb := func(idx, _ int, item list.Item) list.Item {
		hi, ok := item.(list.Highlightable)
		if !ok {
			return item
		}
		if activeRange[0] >= 0 && idx >= activeRange[0] && idx <= activeRange[1] {
			// Inside the range: highlight the entire item.
			hi.SetHighlight(0, 0, -1, -1)
		} else {
			// Outside the range: clear highlight.
			hi.SetHighlight(-1, -1, -1, -1)
		}
		return item
	}
	l.RegisterRenderCallback(cb)

	// First render populates the cache. Each item renders exactly
	// once even though the callback runs for all three.
	_ = l.Render()
	require.Equal(t, 1, a.renderHits, "first render: a renders once")
	require.Equal(t, 1, b.renderHits, "first render: b renders once")
	require.Equal(t, 1, c.renderHits, "first render: c renders once")

	// Subsequent renders without an active range are cache hits.
	// The callback's SetHighlight call dedupes (already cleared),
	// no version bump, frozen entries served verbatim.
	for range 3 {
		_ = l.Render()
	}
	require.Equal(t, 1, a.renderHits, "frozen item must not re-render across stable draws")
	require.Equal(t, 1, b.renderHits, "frozen item must not re-render across stable draws")
	require.Equal(t, 1, c.renderHits, "frozen item must not re-render across stable draws")

	// Activate a selection range over items a and b. The callback
	// will SetHighlight on both during the next render, bumping
	// their versions. The cache hit fails (version mismatch) and
	// each in-range item re-renders exactly once.
	activeRange = [2]int{0, 1}
	_ = l.Render()
	require.Equal(t, 2, a.renderHits, "in-range item must re-render after SetHighlight")
	require.Equal(t, 2, b.renderHits, "in-range item must re-render after SetHighlight")
	require.Equal(t, 1, c.renderHits, "out-of-range item stays frozen")

	// Verify the highlight actually landed on the in-range items.
	sLine, _, eLine, _ := a.Highlight()
	require.Equal(t, 0, sLine)
	require.Equal(t, -1, eLine)
	sLine, _, eLine, _ = c.Highlight()
	require.Equal(t, -1, sLine, "out-of-range item must not be highlighted")
	require.Equal(t, -1, eLine)

	// While the range stays the same, subsequent renders are cache
	// hits. The callback dedupes (same range), no version bump,
	// the post-render entry served verbatim. Note: items are
	// re-frozen because they're still Finished() and not in the
	// list's freezeSuppressed set.
	for range 3 {
		_ = l.Render()
	}
	require.Equal(t, 2, a.renderHits, "in-range item re-freezes after the highlight render")
	require.Equal(t, 2, b.renderHits, "in-range item re-freezes after the highlight render")
	require.Equal(t, 1, c.renderHits, "out-of-range item stays frozen")

	// Move the range off the items entirely. The callback clears
	// each in-range item's highlight back to (-1,-1,-1,-1), which
	// bumps their versions and triggers exactly one re-render
	// each. After that, the entries re-freeze.
	activeRange = [2]int{-1, -1}
	_ = l.Render()
	require.Equal(t, 3, a.renderHits, "exiting-range item must re-render once when highlight clears")
	require.Equal(t, 3, b.renderHits, "exiting-range item must re-render once when highlight clears")
	require.Equal(t, 1, c.renderHits, "never-highlighted item stays frozen")

	// Confirm the highlight has been fully cleared.
	sLine, _, eLine, _ = a.Highlight()
	require.Equal(t, -1, sLine)
	require.Equal(t, -1, eLine)

	// And subsequent renders are cache hits again — the items
	// re-froze.
	for range 3 {
		_ = l.Render()
	}
	require.Equal(t, 3, a.renderHits, "re-frozen item must not re-render across stable draws")
	require.Equal(t, 3, b.renderHits, "re-frozen item must not re-render across stable draws")
	require.Equal(t, 1, c.renderHits, "never-highlighted item stays frozen")
}
