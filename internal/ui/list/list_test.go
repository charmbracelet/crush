package list

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// trackedItem is a test helper that counts Render calls. The body of
// Render is the item's content concatenated with the call counter so
// that "served from cache" vs "freshly rendered" is observable from
// the rendered string itself.
type trackedItem struct {
	*Versioned
	id         string
	body       string
	finished   bool
	renderHits int
}

func newTrackedItem(id, body string, finished bool) *trackedItem {
	return &trackedItem{
		Versioned: NewVersioned(),
		id:        id,
		body:      body,
		finished:  finished,
	}
}

func (t *trackedItem) Render(width int) string {
	t.renderHits++
	return t.body + ":w=" + strconv.Itoa(width)
}

func (t *trackedItem) Finished() bool {
	return t.finished
}

// TestList_RenderMemo_PointerKey covers the F6 invariant that the
// list-level cache is keyed by item pointer, not slice index, so
// PrependItems and AppendItems do not shift cached entries to the
// wrong item.
func TestList_RenderMemo_PointerKey(t *testing.T) {
	t.Parallel()

	a := newTrackedItem("a", "alpha", false)
	b := newTrackedItem("b", "bravo", false)
	c := newTrackedItem("c", "charlie", false)

	l := NewList(a, b, c)
	l.SetSize(40, 10)

	// First render populates the cache for every item.
	first := l.Render()
	require.Equal(t, 1, a.renderHits)
	require.Equal(t, 1, b.renderHits)
	require.Equal(t, 1, c.renderHits)

	// Prepending a new item must not shift the existing entries to
	// the wrong key. The existing items render exactly once more
	// only if their cache was lost, which would be a bug. Scroll to
	// the top so the prepended item is visible and gets rendered.
	z := newTrackedItem("z", "zulu", false)
	l.PrependItems(z)
	l.ScrollToTop()
	_ = l.Render()
	require.Equal(t, 1, z.renderHits, "prepended item rendered once")
	require.Equal(t, 1, a.renderHits, "stable item must keep its cached entry across PrependItems")
	require.Equal(t, 1, b.renderHits, "stable item must keep its cached entry across PrependItems")
	require.Equal(t, 1, c.renderHits, "stable item must keep its cached entry across PrependItems")

	// AppendItems is symmetric.
	d := newTrackedItem("d", "delta", false)
	l.AppendItems(d)
	_ = l.Render()
	require.Equal(t, 1, d.renderHits, "appended item rendered once")
	require.Equal(t, 1, a.renderHits)
	require.Equal(t, 1, b.renderHits)
	require.Equal(t, 1, c.renderHits)

	// The output is non-trivial.
	require.Contains(t, first, "alpha")
}

// TestList_SetSize_WidthChangeInvalidates covers the F6 invariant
// that a width change drops every cached entry but a height-only
// change leaves the cache intact.
func TestList_SetSize_WidthChangeInvalidates(t *testing.T) {
	t.Parallel()

	a := newTrackedItem("a", "alpha", false)
	b := newTrackedItem("b", "bravo", false)

	l := NewList(a, b)
	l.SetSize(40, 10)
	_ = l.Render()
	require.Equal(t, 1, a.renderHits)
	require.Equal(t, 1, b.renderHits)

	// Height-only change: no invalidation.
	l.SetSize(40, 20)
	_ = l.Render()
	require.Equal(t, 1, a.renderHits, "height-only change must keep cache entries")
	require.Equal(t, 1, b.renderHits, "height-only change must keep cache entries")

	// Width change: every entry invalidates.
	l.SetSize(80, 20)
	_ = l.Render()
	require.Equal(t, 2, a.renderHits, "width change must invalidate cache entries")
	require.Equal(t, 2, b.renderHits, "width change must invalidate cache entries")
}

// TestList_RemoveItem_DropsEntry covers the F6 invariant that
// RemoveItem drops the cache entry for the removed item but leaves
// the surviving entries in place.
func TestList_RemoveItem_DropsEntry(t *testing.T) {
	t.Parallel()

	a := newTrackedItem("a", "alpha", false)
	b := newTrackedItem("b", "bravo", false)
	c := newTrackedItem("c", "charlie", false)

	l := NewList(a, b, c)
	l.SetSize(40, 10)
	_ = l.Render()
	require.Equal(t, 1, a.renderHits)
	require.Equal(t, 1, b.renderHits)
	require.Equal(t, 1, c.renderHits)

	l.RemoveItem(1) // remove b
	_ = l.Render()
	// a and c still cached.
	require.Equal(t, 1, a.renderHits, "stable item must keep cached entry across RemoveItem")
	require.Equal(t, 1, c.renderHits, "stable item must keep cached entry across RemoveItem")
	// The removed item's entry is dropped — verify by re-adding b
	// and confirming it renders as if fresh.
	l.AppendItems(b)
	_ = l.Render()
	require.Equal(t, 2, b.renderHits, "re-added item must re-render")
}

// TestList_FrozenItem_NotReRendered covers §4.5.1: items that report
// Finished() == true on entry creation are marked frozen after the
// first render and are never re-rendered until width change or
// version bump.
func TestList_FrozenItem_NotReRendered(t *testing.T) {
	t.Parallel()

	a := newTrackedItem("a", "alpha", true)
	b := newTrackedItem("b", "bravo", true)

	l := NewList(a, b)
	l.SetSize(40, 10)
	_ = l.Render()
	require.Equal(t, 1, a.renderHits, "frozen items render exactly once on first draw")
	require.Equal(t, 1, b.renderHits, "frozen items render exactly once on first draw")

	// Many subsequent renders must not re-render frozen items.
	for range 5 {
		_ = l.Render()
	}
	require.Equal(t, 1, a.renderHits, "frozen items must not re-render across redraws")
	require.Equal(t, 1, b.renderHits, "frozen items must not re-render across redraws")
}

// TestList_FrozenItem_TransitionsAfterFinish covers §4.5.1: a
// streaming item that later reports Finished() == true transitions
// to frozen on the first render after finish.
func TestList_FrozenItem_TransitionsAfterFinish(t *testing.T) {
	t.Parallel()

	a := newTrackedItem("a", "alpha", false) // streaming
	l := NewList(a)
	l.SetSize(40, 10)

	// While unfinished, every render rebuilds the cache because the
	// item's Finished() is false.
	for range 3 {
		// Bump the version to simulate a streaming delta.
		a.Bump()
		_ = l.Render()
	}
	require.Equal(t, 3, a.renderHits)

	// Item finishes; on the next render it freezes.
	a.finished = true
	a.Bump()
	_ = l.Render()
	require.Equal(t, 4, a.renderHits, "post-finish render still happens once")

	for range 5 {
		_ = l.Render()
	}
	require.Equal(t, 4, a.renderHits, "frozen after finish, no further renders")
}

// TestList_FrozenItem_VersionBumpUnfreezes covers §4.5.1: a frozen
// item that gets a version bump (unexpectedly mutated) is unfrozen
// and re-rendered — no stale output.
func TestList_FrozenItem_VersionBumpUnfreezes(t *testing.T) {
	t.Parallel()

	a := newTrackedItem("a", "alpha", true)
	l := NewList(a)
	l.SetSize(40, 10)

	_ = l.Render()
	_ = l.Render()
	require.Equal(t, 1, a.renderHits)

	a.Bump()
	_ = l.Render()
	require.Equal(t, 2, a.renderHits, "version bump must invalidate frozen entry")

	// Re-renders without bumping go back to cache hits.
	_ = l.Render()
	_ = l.Render()
	require.Equal(t, 2, a.renderHits, "post-bump render re-freezes")
}

// TestList_FrozenItem_ResizeUnfreezes covers §4.5.1: resize
// invalidates frozen entries.
func TestList_FrozenItem_ResizeUnfreezes(t *testing.T) {
	t.Parallel()

	a := newTrackedItem("a", "alpha", true)
	l := NewList(a)
	l.SetSize(40, 10)

	_ = l.Render()
	require.Equal(t, 1, a.renderHits)

	l.SetSize(80, 10)
	_ = l.Render()
	require.Equal(t, 2, a.renderHits, "width change must invalidate frozen entry")
}

// TestList_FrozenItem_SelectionDragUnfreeze covers §4.5.1: an active
// selection-drag span must un-freeze items inside the range; ending
// the drag re-freezes them.
func TestList_FrozenItem_SelectionDragUnfreeze(t *testing.T) {
	t.Parallel()

	a := newTrackedItem("a", "alpha", true)
	b := newTrackedItem("b", "bravo", true)
	c := newTrackedItem("c", "charlie", true)

	l := NewList(a, b, c)
	l.SetSize(40, 10)
	_ = l.Render()
	require.Equal(t, 1, a.renderHits)
	require.Equal(t, 1, b.renderHits)
	require.Equal(t, 1, c.renderHits)

	// Begin a selection drag spanning items 0..1. Items inside the
	// range must re-render (they re-render exactly once because
	// the un-freeze drops the cached entry, and the selection
	// suppression keeps them un-frozen until the drag ends).
	l.BeginSelectionDrag(0, 1)
	_ = l.Render()
	require.Equal(t, 2, a.renderHits, "drag-spanned item must re-render once on entering the drag")
	require.Equal(t, 2, b.renderHits, "drag-spanned item must re-render once on entering the drag")
	require.Equal(t, 1, c.renderHits, "out-of-range item must remain frozen")

	// While the drag is active, items inside the range are NOT
	// frozen. Subsequent renders without state changes still
	// trigger re-renders (because version+width hit but frozen=false
	// also matches; we still re-use the cache — no, actually with
	// our implementation we DO cache unfrozen entries by version).
	_ = l.Render()
	require.Equal(t, 2, a.renderHits, "unfrozen but version-stable hits the cache")
	require.Equal(t, 2, b.renderHits, "unfrozen but version-stable hits the cache")

	// End the drag. Items inside the range re-render once and
	// re-freeze.
	l.EndSelectionDrag()
	_ = l.Render()
	require.Equal(t, 3, a.renderHits, "post-drag render re-freezes the entry")
	require.Equal(t, 3, b.renderHits, "post-drag render re-freezes the entry")

	// Subsequent renders are cache hits again.
	for range 3 {
		_ = l.Render()
	}
	require.Equal(t, 3, a.renderHits, "frozen after drag end")
	require.Equal(t, 3, b.renderHits, "frozen after drag end")
}

// TestList_RenderOutputStableAcrossDraws is the F6 byte-equality
// invariant: rendering the same list multiple times must produce the
// same bytes.
func TestList_RenderOutputStableAcrossDraws(t *testing.T) {
	t.Parallel()

	items := make([]Item, 0, 5)
	for i := range 5 {
		items = append(items, newTrackedItem(strconv.Itoa(i), "item-"+strconv.Itoa(i), i%2 == 0))
	}
	l := NewList(items...)
	l.SetSize(40, 20)

	first := l.Render()
	for range 4 {
		require.Equal(t, first, l.Render(), "render output must be byte-stable across draws")
	}
	// And the output is non-trivial.
	require.True(t, strings.Contains(first, "item-0"))
}

// TestList_SetItems_PointerOverlapRetainsCache covers F6 §4.5
// invalidation semantics for SetItems. When the new slice shares
// some pointers with the previous slice (a typical "swap a few
// items, keep the rest" scenario), the cache entries for the
// surviving items must be retained — re-rendering them would defeat
// the memo. Entries for the items that were removed must be
// dropped so they can't serve stale output if the same pointer is
// re-introduced later.
func TestList_SetItems_PointerOverlapRetainsCache(t *testing.T) {
	t.Parallel()

	a := newTrackedItem("a", "alpha", false)
	b := newTrackedItem("b", "bravo", false)
	c := newTrackedItem("c", "charlie", false)
	d := newTrackedItem("d", "delta", false)

	l := NewList(a, b, c)
	l.SetSize(40, 10)
	_ = l.Render()
	require.Equal(t, 1, a.renderHits)
	require.Equal(t, 1, b.renderHits)
	require.Equal(t, 1, c.renderHits)

	// Replace the slice with one that shares a and c (b is dropped,
	// d is added). a and c must keep their cache entries; d renders
	// once on the next draw.
	l.SetItems(a, c, d)
	_ = l.Render()
	require.Equal(t, 1, a.renderHits, "stable item must keep cached entry across SetItems")
	require.Equal(t, 1, c.renderHits, "stable item must keep cached entry across SetItems")
	require.Equal(t, 1, d.renderHits, "new item renders once")

	// Re-introducing b after it was dropped must rebuild its
	// entry (its previous cache entry was invalidated by SetItems).
	l.SetItems(a, b, c)
	_ = l.Render()
	require.Equal(t, 2, b.renderHits, "re-introduced item must re-render — its old entry was dropped")
	// a and c remained throughout both swaps.
	require.Equal(t, 1, a.renderHits, "stable item retained across multiple SetItems")
	require.Equal(t, 1, c.renderHits, "stable item retained across multiple SetItems")
}

// TestList_SetItems_AllNewDropsEveryEntry covers F6 §4.5: when the
// SetItems slice has no pointer overlap with the previous slice,
// every cache entry from the previous slice is dropped. This is
// the pure-replace case (e.g. session switch).
func TestList_SetItems_AllNewDropsEveryEntry(t *testing.T) {
	t.Parallel()

	a := newTrackedItem("a", "alpha", false)
	b := newTrackedItem("b", "bravo", false)
	c := newTrackedItem("c", "charlie", false)

	l := NewList(a, b, c)
	l.SetSize(40, 10)
	_ = l.Render()
	require.Equal(t, 1, a.renderHits)
	require.Equal(t, 1, b.renderHits)
	require.Equal(t, 1, c.renderHits)

	// Replace with a fully disjoint slice. Every entry from the
	// previous slice must be dropped.
	x := newTrackedItem("x", "xray", false)
	y := newTrackedItem("y", "yankee", false)
	l.SetItems(x, y)
	_ = l.Render()
	require.Equal(t, 1, x.renderHits, "new item renders once")
	require.Equal(t, 1, y.renderHits, "new item renders once")

	// Re-introducing the originals must rebuild every entry.
	l.SetItems(a, b, c)
	_ = l.Render()
	require.Equal(t, 2, a.renderHits, "previously-dropped item must re-render")
	require.Equal(t, 2, b.renderHits, "previously-dropped item must re-render")
	require.Equal(t, 2, c.renderHits, "previously-dropped item must re-render")
}

// TestVersioned_BumpMonotonic covers the basic Versioned contract:
// Version() starts at zero and Bump() advances it monotonically.
func TestVersioned_BumpMonotonic(t *testing.T) {
	t.Parallel()

	v := NewVersioned()
	require.Equal(t, uint64(0), v.Version())
	v.Bump()
	require.Equal(t, uint64(1), v.Version())
	v.Bump()
	v.Bump()
	require.Equal(t, uint64(3), v.Version())
}
