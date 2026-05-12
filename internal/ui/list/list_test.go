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

// multiLineItem is a test helper whose Render returns a fixed
// multi-line body. Each line is uniquely identifiable (id:N) so a
// test can reconstruct the expected visible window by index. F7's
// byte-identity matrix is built around these.
type multiLineItem struct {
	*Versioned
	id     string
	height int
}

func newMultiLineItem(id string, height int) *multiLineItem {
	return &multiLineItem{
		Versioned: NewVersioned(),
		id:        id,
		height:    height,
	}
}

func (m *multiLineItem) Render(_ int) string {
	if m.height <= 0 {
		return ""
	}
	parts := make([]string, m.height)
	for i := range m.height {
		parts[i] = m.id + ":" + strconv.Itoa(i)
	}
	return strings.Join(parts, "\n")
}

func (m *multiLineItem) Finished() bool { return true }

// expectedRender computes what list.Render *should* produce from
// first principles given the item heights, viewport, offsetIdx,
// offsetLine, gap, and reverse settings. It mirrors the pre-F7
// "build everything, trim to height, reverse" semantics so we can
// assert byte-identity against the new bounded path.
func expectedRender(items []*multiLineItem, height, offsetIdx, offsetLine, gap int, reverse bool) string {
	if len(items) == 0 {
		return ""
	}
	budget := max(height, 0)
	var lines []string
	currentOffset := offsetLine
	for idx := offsetIdx; idx < len(items) && len(lines) < budget; idx++ {
		body := items[idx].Render(0)
		body = strings.TrimRight(body, "\n")
		itemLines := strings.Split(body, "\n")
		itemHeight := len(itemLines)

		if currentOffset >= 0 && currentOffset < itemHeight {
			lines = append(lines, itemLines[currentOffset:]...)
			for range gap {
				lines = append(lines, "")
			}
		} else {
			gapOffset := currentOffset - itemHeight
			gapRemaining := gap - gapOffset
			for range max(gapRemaining, 0) {
				lines = append(lines, "")
			}
		}
		currentOffset = 0
	}
	if len(lines) > budget {
		lines = lines[:budget]
	}
	if reverse {
		for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
			lines[i], lines[j] = lines[j], lines[i]
		}
	}
	return strings.Join(lines, "\n")
}

// TestList_F7_ByteIdentityMatrix is T1 from the F7 plan: a sweep
// over (item heights × viewport heights × offsets × gaps × reverse)
// that asserts list.Render produces output byte-identical to a
// pre-F7-equivalent reference (build full buffer, trim at end).
func TestList_F7_ByteIdentityMatrix(t *testing.T) {
	t.Parallel()

	itemHeights := [][]int{
		{1},
		{5},
		{1, 1, 1},
		{3, 7, 2},
		{20, 5, 30},
		{50, 1, 50, 1},
	}
	viewportHeights := []int{1, 3, 5, 10, 25, 100}
	offsetIdxs := []int{0, 1, 2}
	offsetLines := []int{0, 1, 4}
	gaps := []int{0, 1, 3}
	reverses := []bool{false, true}

	for _, heights := range itemHeights {
		for _, vh := range viewportHeights {
			for _, oIdx := range offsetIdxs {
				if oIdx >= len(heights) {
					continue
				}
				for _, oLine := range offsetLines {
					maxOffset := heights[oIdx]
					if oLine >= maxOffset {
						continue
					}
					for _, gap := range gaps {
						for _, reverse := range reverses {
							items := make([]*multiLineItem, len(heights))
							asItems := make([]Item, len(heights))
							for i, h := range heights {
								items[i] = newMultiLineItem("i"+strconv.Itoa(i), h)
								asItems[i] = items[i]
							}
							l := NewList(asItems...)
							l.SetSize(40, vh)
							l.SetGap(gap)
							l.SetReverse(reverse)
							l.offsetIdx = oIdx
							l.offsetLine = oLine

							got := l.Render()
							want := expectedRender(items, vh, oIdx, oLine, gap, reverse)
							require.Equalf(t, want, got,
								"mismatch heights=%v vh=%d oIdx=%d oLine=%d gap=%d reverse=%v",
								heights, vh, oIdx, oLine, gap, reverse)
						}
					}
				}
			}
		}
	}
}

// TestList_F7_GiantItemBoundedRender is T2: a single 10,000-line
// item with a 50-line viewport. Render must return exactly 50
// lines — no off-by-one, no trim issue. This is the F7 win in
// test form: per-frame work is bounded by viewport, not item
// height.
func TestList_F7_GiantItemBoundedRender(t *testing.T) {
	t.Parallel()

	const itemHeight = 10000
	const viewport = 50

	giant := newMultiLineItem("giant", itemHeight)
	l := NewList(giant)
	l.SetSize(40, viewport)

	out := l.Render()
	got := strings.Count(out, "\n") + 1
	require.Equal(t, viewport, got, "render output must be exactly viewport lines for an oversized item")

	// And the lines are the prefix of the item starting at
	// offset 0.
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		require.Equal(t, "giant:"+strconv.Itoa(i), line, "line %d does not match expected slice", i)
	}
}

// TestList_F7_GiantItemWithOffsetBoundedRender complements T2 with
// a non-zero offsetLine so we exercise both the "skip prefix" and
// "bound suffix" sides of the slice.
func TestList_F7_GiantItemWithOffsetBoundedRender(t *testing.T) {
	t.Parallel()

	const itemHeight = 10000
	const viewport = 50
	const offset = 1234

	giant := newMultiLineItem("giant", itemHeight)
	l := NewList(giant)
	l.SetSize(40, viewport)
	l.offsetLine = offset

	out := l.Render()
	lines := strings.Split(out, "\n")
	require.Len(t, lines, viewport, "render output must be exactly viewport lines for an oversized item")
	for i, line := range lines {
		require.Equal(t, "giant:"+strconv.Itoa(offset+i), line, "line %d does not match expected slice", i)
	}
}

// TestList_F7_GapOverflow is T3: viewport height 5, two items each
// 10 lines, gap of 3. Render returns exactly 5 lines and never
// includes gap rows beyond the viewport.
func TestList_F7_GapOverflow(t *testing.T) {
	t.Parallel()

	a := newMultiLineItem("a", 10)
	b := newMultiLineItem("b", 10)
	l := NewList(a, b)
	l.SetSize(40, 5)
	l.SetGap(3)

	out := l.Render()
	lines := strings.Split(out, "\n")
	require.Len(t, lines, 5, "viewport must clamp output to height even with gap rows pending")

	// Gap rows after item a would only appear if the viewport
	// extended past the first 10 lines, which it doesn't here.
	for i, line := range lines {
		require.Equal(t, "a:"+strconv.Itoa(i), line, "line %d", i)
	}
}

// TestList_F7_GapOverflow_BoundaryStraddle exercises a viewport
// that lands inside the gap region between two items: 12 lines
// viewport, item a height 10, item b height 10, gap 3 — first 10
// lines from a, then 2 of the 3 gap rows, no b lines yet.
func TestList_F7_GapOverflow_BoundaryStraddle(t *testing.T) {
	t.Parallel()

	a := newMultiLineItem("a", 10)
	b := newMultiLineItem("b", 10)
	l := NewList(a, b)
	l.SetSize(40, 12)
	l.SetGap(3)

	out := l.Render()
	lines := strings.Split(out, "\n")
	require.Len(t, lines, 12)

	for i := range 10 {
		require.Equal(t, "a:"+strconv.Itoa(i), lines[i])
	}
	require.Equal(t, "", lines[10])
	require.Equal(t, "", lines[11])
}

// TestList_F7_ReverseGiantItem is T4: same bounded-slicing
// invariant in reverse mode. Reverse mode keeps the same final
// trim semantics; bounded slicing must produce the same window
// (just reversed).
func TestList_F7_ReverseGiantItem(t *testing.T) {
	t.Parallel()

	const itemHeight = 10000
	const viewport = 50

	giant := newMultiLineItem("giant", itemHeight)
	l := NewList(giant)
	l.SetSize(40, viewport)
	l.SetReverse(true)

	out := l.Render()
	lines := strings.Split(out, "\n")
	require.Len(t, lines, viewport)

	// Expected: the same first-50 slice as the non-reverse path
	// but reversed.
	for i, line := range lines {
		expectedIdx := viewport - 1 - i
		require.Equal(t, "giant:"+strconv.Itoa(expectedIdx), line, "reverse line %d", i)
	}
}

// TestList_F7_OffsetLineAtItemBoundary is T5: offsetLine ==
// itemHeight lands exactly past the last visible line of the item
// at offsetIdx. The renderer must not address line N (which does
// not exist); the visible window starts at the gap rows (when gap
// > 0) or at the next item (when gap == 0).
func TestList_F7_OffsetLineAtItemBoundary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		gap      int
		viewport int
		want     []string
	}{
		{
			name:     "gap zero jumps straight to next item",
			gap:      0,
			viewport: 5,
			want: []string{
				"b:0", "b:1", "b:2", "b:3", "b:4",
			},
		},
		{
			name:     "gap two emits gap rows then next item",
			gap:      2,
			viewport: 5,
			want: []string{
				"", "", "b:0", "b:1", "b:2",
			},
		},
	}

	const itemHeight = 4
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			a := newMultiLineItem("a", itemHeight)
			b := newMultiLineItem("b", 10)
			l := NewList(a, b)
			l.SetSize(40, tc.viewport)
			l.SetGap(tc.gap)
			l.offsetIdx = 0
			l.offsetLine = itemHeight // exactly at the boundary

			out := l.Render()
			require.Equal(t, strings.Join(tc.want, "\n"), out)
		})
	}
}

// TestList_F7_OffsetLineInsideGap is T6: offsetLine is one row
// past the end of the item at offsetIdx, landing inside the gap
// region. The visible window starts at the second gap row and
// then continues into the next item.
func TestList_F7_OffsetLineInsideGap(t *testing.T) {
	t.Parallel()

	const itemHeight = 4
	const gap = 3
	const viewport = 5

	a := newMultiLineItem("a", itemHeight)
	b := newMultiLineItem("b", 10)
	l := NewList(a, b)
	l.SetSize(40, viewport)
	l.SetGap(gap)
	l.offsetIdx = 0
	l.offsetLine = itemHeight + 1 // one row into the gap

	out := l.Render()
	want := strings.Join([]string{
		"",    // second gap row
		"",    // third gap row
		"b:0", // next item starts
		"b:1",
		"b:2",
	}, "\n")
	require.Equal(t, want, out)
}

// TestList_F7_ViewportZeroOrNegative is T7: a non-positive
// viewport height must produce an empty string with no panic and
// must normalize l.height to zero (the budget := max(l.height, 0)
// side effect).
func TestList_F7_ViewportZeroOrNegative(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		height int
	}{
		{name: "height zero", height: 0},
		{name: "height negative", height: -1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			a := newMultiLineItem("a", 5)
			l := NewList(a)
			l.SetSize(40, tc.height)

			out := l.Render()
			require.Equal(t, "", out, "render must be empty for non-positive viewport")
			require.Equal(t, 0, l.height, "render must normalize height to zero")
		})
	}
}
