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

	// totalHeightCache is a cached value of the total rendered height of
	// all items. It is invalidated whenever the item set changes or the
	// viewport width changes (which can alter per-item line counts).
	totalHeightCache int
	totalHeightValid bool

	// cache is the F6 list-level render memo, keyed by item pointer and
	// valid while the item's width and version match.
	cache map[Item]*listCacheEntry

	// dragItems are the items inside an active selection drag, whose
	// entries are dropped when the drag begins and again when it ends.
	dragItems map[Item]struct{}
}

// listCacheEntry is the per-item entry in the list-level render memo.
type listCacheEntry struct {
	width   int
	version uint64
	height  int
	lines   []string
	// stale entries keep an exact height but must re-render to be drawn.
	stale bool
}

// NewList creates a new lazy-loaded list.
func NewList(items ...Item) *List {
	l := new(List)
	l.items = items
	l.selectedIdx = -1
	l.cache = make(map[Item]*listCacheEntry)
	l.dragItems = make(map[Item]struct{})
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

	// Calculate the height from offsetIdx to the end. The comparison is
	// against the visible height (totalHeight minus the lines of the first
	// item that are scrolled out of view), otherwise a first item taller
	// than the viewport reports "not at bottom" while it is in fact
	// pinned there.
	var totalHeight int
	for idx := l.offsetIdx; idx < len(l.items); idx++ {
		if totalHeight-l.offsetLine > l.height {
			// No need to calculate further, we're already past the viewport height
			return false
		}
		itemHeight := l.itemHeight(idx)
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

// TotalHeight returns the total height of all items in the list.
// The result is cached and only recomputed when the item set or
// viewport width changes.
func (l *List) TotalHeight() int {
	if l.totalHeightValid {
		return l.totalHeightCache
	}
	total := 0
	for idx := range l.items {
		total += l.itemHeight(idx)
		if l.gap > 0 && idx < len(l.items)-1 {
			total += l.gap
		}
	}
	l.totalHeightCache = total
	l.totalHeightValid = true
	return total
}

// Prewarm renders items in the range [from, from+batch) into the width
// cache and returns the next index to warm (len(items) when done). It lets
// a caller populate the per-item render cache incrementally across frames
// so a later TotalHeight is instant instead of rendering everything at
// once. Rendering is otherwise identical to what TotalHeight would do.
func (l *List) Prewarm(from, batch int) int {
	if from < 0 {
		from = 0
	}
	end := min(from+batch, len(l.items))
	for idx := from; idx < end; idx++ {
		l.itemHeight(idx)
	}
	return end
}

// Overflows reports whether the items' total height exceeds the given
// viewport height. It walks from the bottom and stops as soon as the
// threshold is crossed, so for content taller than the viewport (the
// common case) it renders only a viewport's worth of items rather than
// all of them — much cheaper than TotalHeight when only the boolean is
// needed (e.g. deciding whether a scrollbar is required).
func (l *List) Overflows(height int) bool {
	total := 0
	for idx := len(l.items) - 1; idx >= 0; idx-- {
		total += l.itemHeight(idx)
		if l.gap > 0 && idx < len(l.items)-1 {
			total += l.gap
		}
		if total > height {
			return true
		}
	}
	return false
}

// ItemsVersion folds every item's version into one number. It changes
// whenever any item in this list mutates in a way that affects its rendered
// output, since that is exactly the contract [Versioned.Bump] carries.
//
// It reads one field per item and renders nothing, so it is cheap enough to
// call once per frame. Callers that memoize a whole rendered frame fold it
// into their cache key to pick up item mutations they do not otherwise know
// about.
func (l *List) ItemsVersion() uint64 {
	var v uint64
	for _, item := range l.items {
		v = v*31 + item.Version()
	}
	return v
}

// ScrollPosition returns the index of the first visible item and the line
// offset into it. Unlike Offset it is O(1) and does not render items.
func (l *List) ScrollPosition() (offsetIdx, offsetLine int) {
	return l.offsetIdx, l.offsetLine
}

// Offset returns the current scroll offset in lines from the top.
func (l *List) Offset() int {
	offset := 0
	for idx := 0; idx < l.offsetIdx; idx++ {
		offset += l.itemHeight(idx)
		if l.gap > 0 && idx < len(l.items)-1 {
			offset += l.gap
		}
	}
	offset += l.offsetLine
	return offset
}

// lastOffsetItem returns the index and line offsets of the last item that can
// be partially visible in the viewport.
func (l *List) lastOffsetItem() (int, int, int) {
	var totalHeight int
	var idx int
	for idx = len(l.items) - 1; idx >= 0; idx-- {
		itemHeight := l.itemHeight(idx)
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

// itemHeight returns the height of the item at the given index, rendering
// it only when it has no current entry. A stale entry's height is exact.
func (l *List) itemHeight(idx int) int {
	item, entry := l.lookup(idx)
	if item == nil {
		return 0
	}
	if entry == nil {
		entry = l.render(idx, item)
	}
	return entry.height
}

// drawEntry returns a current, drawable entry for the given index,
// rendering it on a miss or when the entry is stale. The result must not
// be retained past the next invalidation (SetSize width change, SetItems,
// etc.).
func (l *List) drawEntry(idx int) *listCacheEntry {
	item, entry := l.lookup(idx)
	if item == nil {
		return nil
	}
	if entry == nil || entry.stale {
		entry = l.render(idx, item)
	}
	return entry
}

// lookup runs the render callbacks for the item at idx and returns the
// item to render along with its cache entry, or a nil entry when that is
// out of date. Callbacks always run: they are how the list discovers
// per-frame state (selection, highlight), and they bump the item's
// version when that state changes its output.
func (l *List) lookup(idx int) (Item, *listCacheEntry) {
	if idx < 0 || idx >= len(l.items) {
		return nil, nil
	}
	rawItem := l.items[idx]
	item := rawItem
	for _, cb := range l.renderCallbacks {
		if it := cb(idx, l.selectedIdx, item); it != nil {
			item = it
		}
	}
	entry := l.cache[rawItem]
	if entry == nil || entry.width != l.width || entry.version != rawItem.Version() {
		return item, nil
	}
	return item, entry
}

// render renders item, the callback-adjusted form of the item at idx, and
// stores the result in the cache.
func (l *List) render(idx int, item Item) *listCacheEntry {
	rawItem := l.items[idx]
	rendered := strings.TrimRight(item.Render(l.width), "\n")
	lines := strings.Split(rendered, "\n")

	entry := l.cache[rawItem]
	if entry == nil {
		entry = &listCacheEntry{}
		l.cache[rawItem] = entry
	}
	if entry.height != len(lines) {
		l.totalHeightValid = false
	}
	// The version is read after Render so a bump Render itself causes is
	// not mistaken for a later mutation.
	*entry = listCacheEntry{width: l.width, version: rawItem.Version(), height: len(lines), lines: lines}
	return entry
}

// invalidateAll drops every cache entry. Called on width changes.
func (l *List) invalidateAll() {
	for k := range l.cache {
		delete(l.cache, k)
	}
	l.totalHeightValid = false
}

// Invalidate drops the cache entry for the given item, forcing a re-render
// on next use. No-op if the item is not in the cache.
func (l *List) Invalidate(item Item) {
	delete(l.cache, item)
}

// Restyle runs restyle on every item for a change that affects colours but
// not layout, such as a theme swap. Cached heights that were current stay
// valid, so scroll geometry needs no re-render; each item re-renders its
// content once it is drawn.
func (l *List) Restyle(restyle func(Item)) {
	for _, item := range l.items {
		entry := l.cache[item]
		current := entry != nil && entry.width == l.width && entry.version == item.Version()
		restyle(item)
		if current {
			entry.version = item.Version()
			entry.stale = true
		}
	}
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

// BeginSelectionDrag drops the cached entries of the items in the
// inclusive [startIdx, endIdx] range, so they re-render with the live
// selection overlay. EndSelectionDrag drops them again for a clean render.
// Indices outside the items slice are clipped silently.
func (l *List) BeginSelectionDrag(startIdx, endIdx int) {
	if len(l.items) == 0 {
		return
	}
	if startIdx > endIdx {
		startIdx, endIdx = endIdx, startIdx
	}
	startIdx = max(startIdx, 0)
	endIdx = min(endIdx, len(l.items)-1)
	for _, it := range l.items[startIdx : endIdx+1] {
		l.dragItems[it] = struct{}{}
		delete(l.cache, it)
	}
}

// EndSelectionDrag drops the entries of the items the drag covered so they
// re-render without the selection overlay.
func (l *List) EndSelectionDrag() {
	for it := range l.dragItems {
		delete(l.dragItems, it)
		delete(l.cache, it)
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
		currentHeight := l.itemHeight(l.offsetIdx)
		for l.offsetLine >= currentHeight {
			l.offsetLine -= currentHeight
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
			currentHeight = l.itemHeight(l.offsetIdx)
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
			totalHeight := l.itemHeight(l.offsetIdx)
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
		visibleHeight += l.itemHeight(currentIdx)
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
	lines := l.renderWindow(budget)
	// A window that ends before the viewport is full while scrolled means
	// the offset is past the end: content below it shrank or went away
	// (an item removed, a tail that collapsed, a filter). Clamp so the
	// view never sits lifted off the bottom. Checking only the underfilled
	// case keeps the common path free of the tail walk.
	if len(lines) < budget && (l.offsetIdx > 0 || l.offsetLine > 0) {
		l.ScrollToBottom()
		lines = l.renderWindow(budget)
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

// renderWindow collects at most budget lines starting at the scroll offset.
func (l *List) renderWindow(budget int) []string {
	lines := make([]string, 0, budget)
	currentIdx := l.offsetIdx
	currentOffset := l.offsetLine

	for currentIdx < len(l.items) {
		remaining := budget - len(lines)
		if remaining <= 0 {
			break
		}

		entry := l.drawEntry(currentIdx)
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
	return lines
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
	l.totalHeightValid = false
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
	l.totalHeightValid = false
}

// AppendItems appends items to the list.
func (l *List) AppendItems(items ...Item) {
	l.items = append(l.items, items...)
	l.totalHeightValid = false
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
	delete(l.dragItems, removed)

	// Adjust selection if needed
	if l.selectedIdx == idx {
		l.selectedIdx = -1
	} else if l.selectedIdx > idx {
		l.selectedIdx--
	}

	// Adjust offset if needed. Removing the item the view starts inside
	// starts the view at the top of the item that slid into its place; the
	// old line offset belonged to the removed item and would skip into,
	// or past, the new one.
	if l.offsetIdx > idx {
		l.offsetIdx--
	} else if l.offsetIdx == idx {
		l.offsetIdx = min(l.offsetIdx, max(0, len(l.items)-1))
		l.offsetLine = 0
	}
	l.totalHeightValid = false
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

	// The list may not have been sized yet when the caller sets up its
	// selection, e.g. a dialog constructor that runs before the first
	// Draw. With no viewport height there is no visibility window to fit
	// the selection into, so pin the selected item to the top of the
	// viewport; the first render then shows it instead of computing a
	// bogus offset that skips past it entirely.
	if l.height <= 0 {
		l.offsetIdx = l.selectedIdx
		l.offsetLine = 0
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
			totalHeight += l.itemHeight(i)
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
		itemEndLine := currentLine + l.itemHeight(currentIdx)

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
