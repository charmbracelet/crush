package list

import (
	"strings"
)

// List represents a list of items that can be lazily rendered. A list is
// always rendered like a chat conversation where items are stacked vertically
// from top to bottom.
type List struct {
	// Viewport size
	width, height int

	// Items in the list
	items []Item

	// Gap between items (0 or less means no gap)
	gap int

	// show list in reverse order
	reverse bool

	// Focus and selection state
	focused     bool
	selectedIdx int // The current selected index -1 means no selection

	// offsetIdx is the index of the first visible item in the viewport.
	offsetIdx int
	// offsetLine is the number of lines of the item at offsetIdx that are
	// scrolled out of view (above the viewport).
	// It must always be >= 0.
	offsetLine int

	// renderCallbacks is a list of callbacks to apply when rendering items.
	renderCallbacks []func(idx, selectedIdx int, item Item) Item

	// cache is the F6 list-level render memo, keyed by item pointer.
	// Each entry stores the rendered content, a pre-split slice of
	// lines (so AtBottom / Render / VisibleItemIndices /
	// findItemAtY all share one render per frame), the height, and
	// the keys that govern invalidation (width and version). The
	// frozen flag mirrors §4.5.1: once a Finished() item is
	// rendered, subsequent draws return the stored output verbatim
	// without calling back into Render.
	cache map[Item]*listCacheEntry

	// freezeSuppressed marks items the list must not freeze on the
	// next render even when their Finished() reports true. This is
	// the §4.5.1 selection-drag escape hatch (option (a)): items
	// inside an active selection range render as live items so that
	// per-line highlight overlays land on the latest content. Cleared
	// on EndSelectionDrag.
	freezeSuppressed map[Item]struct{}
}

// listCacheEntry is the per-item entry in the list-level render memo.
type listCacheEntry struct {
	width   int
	version uint64
	frozen  bool
	content string
	lines   []string
	height  int
}

// renderedItem is the legacy view of a cached entry returned by getItem.
// Internal callers that don't need the line slice keep using this
// shape; functions that walk lines (Render) take the slice off the
// cache entry directly.
type renderedItem struct {
	content string
	height  int
}

// NewList creates a new lazy-loaded list.
func NewList(items ...Item) *List {
	l := new(List)
	l.items = items
	l.selectedIdx = -1
	l.cache = make(map[Item]*listCacheEntry)
	l.freezeSuppressed = make(map[Item]struct{})
	return l
}

// RenderCallback defines a function that can modify an item before it is
// rendered.
type RenderCallback func(idx, selectedIdx int, item Item) Item

// RegisterRenderCallback registers a callback to be called when rendering
// items. This can be used to modify items before they are rendered.
func (l *List) RegisterRenderCallback(cb RenderCallback) {
	l.renderCallbacks = append(l.renderCallbacks, cb)
}

// SetSize sets the size of the list viewport. A width change drops the
// entire render cache because every entry's wrapped output depends on
// width; a height-only change is a no-op for the cache.
func (l *List) SetSize(width, height int) {
	if l.width != width {
		l.invalidateAll()
	}
	l.width = width
	l.height = height
}

// SetGap sets the gap between items.
func (l *List) SetGap(gap int) {
	l.gap = gap
}

// Gap returns the gap between items.
func (l *List) Gap() int {
	return l.gap
}

// AtBottom returns whether the list is showing the last item at the bottom.
func (l *List) AtBottom() bool {
	if len(l.items) == 0 {
		return true
	}

	// Calculate the height from offsetIdx to the end.
	var totalHeight int
	for idx := l.offsetIdx; idx < len(l.items); idx++ {
		if totalHeight > l.height {
			// No need to calculate further, we're already past the viewport height
			return false
		}
		item := l.getItem(idx)
		itemHeight := item.height
		if l.gap > 0 && idx > l.offsetIdx {
			itemHeight += l.gap
		}
		totalHeight += itemHeight
	}

	return totalHeight-l.offsetLine <= l.height
}

// SetReverse shows the list in reverse order.
func (l *List) SetReverse(reverse bool) {
	l.reverse = reverse
}

// Width returns the width of the list viewport.
func (l *List) Width() int {
	return l.width
}

// Height returns the height of the list viewport.
func (l *List) Height() int {
	return l.height
}

// Len returns the number of items in the list.
func (l *List) Len() int {
	return len(l.items)
}

// lastOffsetItem returns the index and line offsets of the last item that can
// be partially visible in the viewport.
func (l *List) lastOffsetItem() (int, int, int) {
	var totalHeight int
	var idx int
	for idx = len(l.items) - 1; idx >= 0; idx-- {
		item := l.getItem(idx)
		itemHeight := item.height
		if l.gap > 0 && idx < len(l.items)-1 {
			itemHeight += l.gap
		}
		totalHeight += itemHeight
		if totalHeight > l.height {
			break
		}
	}

	// Calculate line offset within the item
	lineOffset := max(totalHeight-l.height, 0)
	idx = max(idx, 0)

	return idx, lineOffset, totalHeight
}

// getItem renders (if needed) and returns the item at the given index.
// The result is served from the F6 cache when possible — see
// renderItemEntry for the cache-key semantics.
func (l *List) getItem(idx int) renderedItem {
	if idx < 0 || idx >= len(l.items) {
		return renderedItem{}
	}
	entry := l.renderItemEntry(idx)
	if entry == nil {
		return renderedItem{}
	}
	return renderedItem{content: entry.content, height: entry.height}
}

// renderItemEntry returns the cache entry for the given index, populating
// the cache on miss. The result must not be retained past the next
// invalidation (SetSize width change, SetItems, etc.).
//
// Render callbacks always run, even for frozen entries: callbacks
// are how the list discovers per-frame state changes (selection,
// highlight range) and they bump the item's version when those
// changes affect the rendered output. A frozen item whose callback
// run is a no-op (same focus, same highlight) keeps its stored
// version and the cache hit is preserved on the post-callback
// version check.
func (l *List) renderItemEntry(idx int) *listCacheEntry {
	if idx < 0 || idx >= len(l.items) {
		return nil
	}

	rawItem := l.items[idx]
	entry := l.cache[rawItem]

	// Run render callbacks. Callbacks may mutate the item (focus,
	// highlight) which in turn bumps its version when state actually
	// changes. We capture the post-callback version below.
	item := rawItem
	if len(l.renderCallbacks) > 0 {
		for _, cb := range l.renderCallbacks {
			if it := cb(idx, l.selectedIdx, item); it != nil {
				item = it
			}
		}
	}

	version := rawItem.Version()
	if entry != nil && entry.width == l.width && entry.version == version {
		// Cache hit — frozen or unfrozen, the entry content is
		// still correct because no version bump landed since the
		// last render. Selection-drag suppression turns this into
		// a miss only if the entry is frozen.
		if !entry.frozen {
			return entry
		}
		if _, suppressed := l.freezeSuppressed[rawItem]; !suppressed {
			return entry
		}
	}

	rendered := item.Render(l.width)
	rendered = strings.TrimRight(rendered, "\n")
	lines := strings.Split(rendered, "\n")
	height := len(lines)

	// Re-read the version after Render so that any version bumps
	// caused by Render itself (e.g. an item that mutates internal
	// state during rendering) are captured. Without this we would
	// freeze a stale entry under the post-render version.
	finalVersion := rawItem.Version()

	frozen := false
	if rawItem.Finished() {
		if _, suppressed := l.freezeSuppressed[rawItem]; !suppressed {
			frozen = true
		}
	}

	if entry == nil {
		entry = &listCacheEntry{}
		l.cache[rawItem] = entry
	}
	entry.width = l.width
	entry.version = finalVersion
	entry.frozen = frozen
	entry.content = rendered
	entry.lines = lines
	entry.height = height
	return entry
}

// invalidateAll drops every cache entry. Called on width changes.
func (l *List) invalidateAll() {
	for k := range l.cache {
		delete(l.cache, k)
	}
}

// Invalidate drops the cache entry for the given item, forcing a
// re-render on the next getItem call. No-op if the item is not in
// the cache.
func (l *List) Invalidate(item Item) {
	delete(l.cache, item)
}

// InvalidateFrozen drops the frozen flag (and stored content) for the
// given item. Equivalent to Invalidate but exposed under the F6
// frozen-items vocabulary so external callers can express intent.
func (l *List) InvalidateFrozen(item Item) {
	delete(l.cache, item)
}

// retainCacheFor drops every cache entry whose key is not in the given
// item set. Used by SetItems to keep entries for stable items while
// dropping entries for removed ones.
func (l *List) retainCacheFor(items []Item) {
	if len(l.cache) == 0 {
		return
	}
	keep := make(map[Item]struct{}, len(items))
	for _, it := range items {
		keep[it] = struct{}{}
	}
	for k := range l.cache {
		if _, ok := keep[k]; !ok {
			delete(l.cache, k)
		}
	}
}

// BeginSelectionDrag marks the items in the inclusive [startIdx, endIdx]
// range as un-freezable for the duration of an active selection drag.
// Frozen entries inside the range are dropped so the next render
// reflects live selection-overlay output. The corresponding
// EndSelectionDrag clears the suppression set and lets items
// re-freeze on their next render. Indices outside the items slice
// are clipped silently.
func (l *List) BeginSelectionDrag(startIdx, endIdx int) {
	if len(l.items) == 0 {
		return
	}
	if startIdx > endIdx {
		startIdx, endIdx = endIdx, startIdx
	}
	startIdx = max(startIdx, 0)
	endIdx = min(endIdx, len(l.items)-1)
	for i := startIdx; i <= endIdx; i++ {
		it := l.items[i]
		l.freezeSuppressed[it] = struct{}{}
		// Drop any cached frozen entry so the next render rebuilds
		// it as a live (un-frozen) entry that picks up the
		// selection overlay.
		if entry, ok := l.cache[it]; ok && entry.frozen {
			delete(l.cache, it)
		}
	}
}

// EndSelectionDrag clears the selection-drag freeze suppression. Items
// inside the previous range will re-freeze on their next render once
// their Finished() reports true again.
func (l *List) EndSelectionDrag() {
	for k := range l.freezeSuppressed {
		delete(l.freezeSuppressed, k)
		// Drop the cache entry so the next render produces a clean
		// (un-highlighted) frozen entry.
		delete(l.cache, k)
	}
}

// ScrollToIndex scrolls the list to the given item index.
func (l *List) ScrollToIndex(index int) {
	if index < 0 {
		index = 0
	}
	if index >= len(l.items) {
		index = len(l.items) - 1
	}
	l.offsetIdx = index
	l.offsetLine = 0
}

// ScrollBy scrolls the list by the given number of lines.
func (l *List) ScrollBy(lines int) {
	if len(l.items) == 0 || lines == 0 {
		return
	}

	if l.reverse {
		lines = -lines
	}

	if lines > 0 {
		if l.AtBottom() {
			// Already at bottom
			return
		}

		// Scroll down
		l.offsetLine += lines
		currentItem := l.getItem(l.offsetIdx)
		for l.offsetLine >= currentItem.height {
			l.offsetLine -= currentItem.height
			if l.gap > 0 {
				l.offsetLine = max(0, l.offsetLine-l.gap)
			}

			// Move to next item
			l.offsetIdx++
			if l.offsetIdx > len(l.items)-1 {
				// Reached bottom
				l.ScrollToBottom()
				return
			}
			currentItem = l.getItem(l.offsetIdx)
		}

		lastOffsetIdx, lastOffsetLine, _ := l.lastOffsetItem()
		if l.offsetIdx > lastOffsetIdx || (l.offsetIdx == lastOffsetIdx && l.offsetLine > lastOffsetLine) {
			// Clamp to bottom
			l.offsetIdx = lastOffsetIdx
			l.offsetLine = lastOffsetLine
		}
	} else if lines < 0 {
		// Scroll up
		l.offsetLine += lines // lines is negative
		for l.offsetLine < 0 {
			// Move to previous item
			l.offsetIdx--
			if l.offsetIdx < 0 {
				// Reached top
				l.ScrollToTop()
				break
			}
			prevItem := l.getItem(l.offsetIdx)
			totalHeight := prevItem.height
			if l.gap > 0 {
				totalHeight += l.gap
			}
			l.offsetLine += totalHeight
		}
	}
}

// VisibleItemIndices finds the range of items that are visible in the viewport.
// This is used for checking if selected item is in view.
func (l *List) VisibleItemIndices() (startIdx, endIdx int) {
	if len(l.items) == 0 {
		return 0, 0
	}

	startIdx = l.offsetIdx
	currentIdx := startIdx
	visibleHeight := -l.offsetLine

	for currentIdx < len(l.items) {
		item := l.getItem(currentIdx)
		visibleHeight += item.height
		if l.gap > 0 {
			visibleHeight += l.gap
		}

		if visibleHeight >= l.height {
			break
		}
		currentIdx++
	}

	endIdx = currentIdx
	if endIdx >= len(l.items) {
		endIdx = len(l.items) - 1
	}

	return startIdx, endIdx
}

// Render renders the list and returns the visible lines.
//
// F7: per-item slicing is bounded by the remaining viewport budget so
// per-frame work is O(viewport) rather than O(total item heights).
// We never append beyond l.height lines to the output buffer; the
// final trim is therefore unnecessary. Reverse mode applies the same
// final reversal as before, which is byte-identical because the
// pre-F7 trim happened at the tail of the joined buffer (the same
// lines we now drop implicitly per item).
func (l *List) Render() string {
	if len(l.items) == 0 {
		return ""
	}

	budget := max(l.height, 0)
	lines := make([]string, 0, budget)
	currentIdx := l.offsetIdx
	currentOffset := l.offsetLine

	for currentIdx < len(l.items) {
		remaining := budget - len(lines)
		if remaining <= 0 {
			break
		}

		entry := l.renderItemEntry(currentIdx)
		if entry == nil {
			break
		}
		itemLines := entry.lines
		itemHeight := len(itemLines)

		if currentOffset >= 0 && currentOffset < itemHeight {
			// Append only the visible slice that fits in the
			// remaining viewport budget. Anything past the
			// budget would be discarded by the pre-F7 tail
			// trim, so skipping the append here is
			// byte-identical and bounded.
			visible := itemLines[currentOffset:]
			if len(visible) > remaining {
				visible = visible[:remaining]
			}
			lines = append(lines, visible...)

			// Gap rows after the item, capped to the
			// remaining budget so a 30k-line item with a
			// trailing gap can't push past the viewport.
			if l.gap > 0 {
				gapBudget := min(budget-len(lines), l.gap)
				for range gapBudget {
					lines = append(lines, "")
				}
			}
		} else {
			// offsetLine starts inside the gap.
			gapOffset := currentOffset - itemHeight
			gapRemaining := l.gap - gapOffset
			if gapRemaining > 0 {
				gapBudget := min(budget-len(lines), gapRemaining)
				for range gapBudget {
					lines = append(lines, "")
				}
			}
		}

		currentIdx++
		currentOffset = 0 // Reset offset for subsequent items.
	}

	l.height = budget

	if l.reverse {
		// Reverse the lines so the list renders bottom-to-top.
		for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
			lines[i], lines[j] = lines[j], lines[i]
		}
	}

	return strings.Join(lines, "\n")
}

// PrependItems prepends items to the list.
func (l *List) PrependItems(items ...Item) {
	l.items = append(items, l.items...)

	// Keep view position relative to the content that was visible
	l.offsetIdx += len(items)

	// Update selection index if valid
	if l.selectedIdx != -1 {
		l.selectedIdx += len(items)
	}
}

// SetItems sets the items in the list. Cache entries for items that
// remain after the swap are preserved; entries for removed items are
// dropped.
func (l *List) SetItems(items ...Item) {
	l.items = items
	l.selectedIdx = min(l.selectedIdx, len(l.items)-1)
	l.offsetIdx = min(l.offsetIdx, len(l.items)-1)
	l.offsetLine = 0
	l.retainCacheFor(items)
}

// AppendItems appends items to the list.
func (l *List) AppendItems(items ...Item) {
	l.items = append(l.items, items...)
}

// RemoveItem removes the item at the given index from the list.
func (l *List) RemoveItem(idx int) {
	if idx < 0 || idx >= len(l.items) {
		return
	}

	removed := l.items[idx]

	// Remove the item
	l.items = append(l.items[:idx], l.items[idx+1:]...)

	// Drop the cache entry for the removed item; entries for stable
	// items stay valid because they are keyed by pointer, not index.
	delete(l.cache, removed)
	delete(l.freezeSuppressed, removed)

	// Adjust selection if needed
	if l.selectedIdx == idx {
		l.selectedIdx = -1
	} else if l.selectedIdx > idx {
		l.selectedIdx--
	}

	// Adjust offset if needed
	if l.offsetIdx > idx {
		l.offsetIdx--
	} else if l.offsetIdx == idx && l.offsetIdx >= len(l.items) {
		l.offsetIdx = max(0, len(l.items)-1)
		l.offsetLine = 0
	}
}

// Focused returns whether the list is focused.
func (l *List) Focused() bool {
	return l.focused
}

// Focus sets the focus state of the list.
func (l *List) Focus() {
	l.focused = true
}

// Blur removes the focus state from the list.
func (l *List) Blur() {
	l.focused = false
}

// ScrollToTop scrolls the list to the top.
func (l *List) ScrollToTop() {
	l.offsetIdx = 0
	l.offsetLine = 0
}

// ScrollToBottom scrolls the list to the bottom.
func (l *List) ScrollToBottom() {
	if len(l.items) == 0 {
		return
	}

	lastOffsetIdx, lastOffsetLine, _ := l.lastOffsetItem()
	l.offsetIdx = lastOffsetIdx
	l.offsetLine = lastOffsetLine
}

// ScrollToSelected scrolls the list to the selected item.
func (l *List) ScrollToSelected() {
	if l.selectedIdx < 0 || l.selectedIdx >= len(l.items) {
		return
	}

	startIdx, endIdx := l.VisibleItemIndices()
	if l.selectedIdx < startIdx {
		// Selected item is above the visible range
		l.offsetIdx = l.selectedIdx
		l.offsetLine = 0
	} else if l.selectedIdx > endIdx {
		// Selected item is below the visible range
		// Scroll so that the selected item is at the bottom
		var totalHeight int
		for i := l.selectedIdx; i >= 0; i-- {
			item := l.getItem(i)
			totalHeight += item.height
			if l.gap > 0 && i < l.selectedIdx {
				totalHeight += l.gap
			}
			if totalHeight >= l.height {
				l.offsetIdx = i
				l.offsetLine = totalHeight - l.height
				break
			}
		}
		if totalHeight < l.height {
			// All items fit in the viewport
			l.ScrollToTop()
		}
	}
}

// SelectedItemInView returns whether the selected item is currently in view.
func (l *List) SelectedItemInView() bool {
	if l.selectedIdx < 0 || l.selectedIdx >= len(l.items) {
		return false
	}
	startIdx, endIdx := l.VisibleItemIndices()
	return l.selectedIdx >= startIdx && l.selectedIdx <= endIdx
}

// SetSelected sets the selected item index in the list.
// It returns -1 if the index is out of bounds.
func (l *List) SetSelected(index int) {
	if index < 0 || index >= len(l.items) {
		l.selectedIdx = -1
	} else {
		l.selectedIdx = index
	}
}

// Selected returns the index of the currently selected item. It returns -1 if
// no item is selected.
func (l *List) Selected() int {
	return l.selectedIdx
}

// IsSelectedFirst returns whether the first item is selected.
func (l *List) IsSelectedFirst() bool {
	return l.selectedIdx == 0
}

// IsSelectedLast returns whether the last item is selected.
func (l *List) IsSelectedLast() bool {
	return l.selectedIdx == len(l.items)-1
}

// SelectPrev selects the visually previous item (moves toward visual top).
// It returns whether the selection changed.
func (l *List) SelectPrev() bool {
	if l.reverse {
		// In reverse, visual up = higher index
		if l.selectedIdx < len(l.items)-1 {
			l.selectedIdx++
			return true
		}
	} else {
		// Normal: visual up = lower index
		if l.selectedIdx > 0 {
			l.selectedIdx--
			return true
		}
	}
	return false
}

// SelectNext selects the next item in the list.
// It returns whether the selection changed.
func (l *List) SelectNext() bool {
	if l.reverse {
		// In reverse, visual down = lower index
		if l.selectedIdx > 0 {
			l.selectedIdx--
			return true
		}
	} else {
		// Normal: visual down = higher index
		if l.selectedIdx < len(l.items)-1 {
			l.selectedIdx++
			return true
		}
	}
	return false
}

// SelectFirst selects the first item in the list.
// It returns whether the selection changed.
func (l *List) SelectFirst() bool {
	if len(l.items) == 0 {
		return false
	}
	l.selectedIdx = 0
	return true
}

// SelectLast selects the last item in the list (highest index).
// It returns whether the selection changed.
func (l *List) SelectLast() bool {
	if len(l.items) == 0 {
		return false
	}
	l.selectedIdx = len(l.items) - 1
	return true
}

// WrapToStart wraps selection to the visual start (for circular navigation).
// In normal mode, this is index 0. In reverse mode, this is the highest index.
func (l *List) WrapToStart() bool {
	if len(l.items) == 0 {
		return false
	}
	if l.reverse {
		l.selectedIdx = len(l.items) - 1
	} else {
		l.selectedIdx = 0
	}
	return true
}

// WrapToEnd wraps selection to the visual end (for circular navigation).
// In normal mode, this is the highest index. In reverse mode, this is index 0.
func (l *List) WrapToEnd() bool {
	if len(l.items) == 0 {
		return false
	}
	if l.reverse {
		l.selectedIdx = 0
	} else {
		l.selectedIdx = len(l.items) - 1
	}
	return true
}

// SelectedItem returns the currently selected item. It may be nil if no item
// is selected.
func (l *List) SelectedItem() Item {
	if l.selectedIdx < 0 || l.selectedIdx >= len(l.items) {
		return nil
	}
	return l.items[l.selectedIdx]
}

// SelectFirstInView selects the first item currently in view.
func (l *List) SelectFirstInView() {
	startIdx, _ := l.VisibleItemIndices()
	l.selectedIdx = startIdx
}

// SelectLastInView selects the last item currently in view.
func (l *List) SelectLastInView() {
	_, endIdx := l.VisibleItemIndices()
	l.selectedIdx = endIdx
}

// ItemAt returns the item at the given index.
func (l *List) ItemAt(index int) Item {
	if index < 0 || index >= len(l.items) {
		return nil
	}
	return l.items[index]
}

// ItemIndexAtPosition returns the item at the given viewport-relative y
// coordinate. Returns the item index and the y offset within that item. It
// returns -1, -1 if no item is found.
func (l *List) ItemIndexAtPosition(x, y int) (itemIdx int, itemY int) {
	return l.findItemAtY(x, y)
}

// findItemAtY finds the item at the given viewport y coordinate.
// Returns the item index and the y offset within that item. It returns -1, -1
// if no item is found.
func (l *List) findItemAtY(_, y int) (itemIdx int, itemY int) {
	if y < 0 || y >= l.height {
		return -1, -1
	}

	// Walk through visible items to find which one contains this y
	currentIdx := l.offsetIdx
	currentLine := -l.offsetLine // Negative because offsetLine is how many lines are hidden

	for currentIdx < len(l.items) && currentLine < l.height {
		item := l.getItem(currentIdx)
		itemEndLine := currentLine + item.height

		// Check if y is within this item's visible range
		if y >= currentLine && y < itemEndLine {
			// Found the item, calculate itemY (offset within the item)
			itemY = y - currentLine
			return currentIdx, itemY
		}

		// Move to next item
		currentLine = itemEndLine
		if l.gap > 0 {
			currentLine += l.gap
		}
		currentIdx++
	}

	return -1, -1
}
