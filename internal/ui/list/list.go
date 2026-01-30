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
}

// renderedItem holds the rendered content and height of an item.
type renderedItem struct {
	content string
	height  int
}

// NewList creates a new lazy-loaded list.
func NewList(items ...Item) *List {
	l := new(List)
	l.items = items
	l.selectedIdx = -1
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

// SetSize sets the size of the list viewport.
func (l *List) SetSize(width, height int) {
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
	const margin = 2

	if len(l.items) == 0 {
		return true
	}

	// Calculate the height from offsetIdx to the end.
	var totalHeight int
	for idx := l.offsetIdx; idx < len(l.items); idx++ {
		item := l.getItem(idx)
		itemHeight := item.height
		if l.gap > 0 && idx > l.offsetIdx {
			itemHeight += l.gap
		}
		totalHeight += itemHeight
	}

	return totalHeight-l.offsetLine-margin <= l.height
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
func (l *List) getItem(idx int) renderedItem {
	if idx < 0 || idx >= len(l.items) {
		return renderedItem{}
	}

	item := l.items[idx]
	if len(l.renderCallbacks) > 0 {
		for _, cb := range l.renderCallbacks {
			if it := cb(idx, l.selectedIdx, item); it != nil {
				item = it
			}
		}
	}

	rendered := item.Render(l.width)
	rendered = strings.TrimRight(rendered, "\n")
	height := countLines(rendered)
	ri := renderedItem{
		content: rendered,
		height:  height,
	}

	return ri
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
		// Scroll down
		l.offsetLine += lines
		currentItem := l.getItem(l.offsetIdx)
		for l.offsetLine >= currentItem.height {
			l.offsetLine -= currentItem.height
			if l.gap > 0 {
				l.offsetLine -= l.gap
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
			if l.offsetIdx <= 0 {
				// Reached top
				l.ScrollToTop()
				break
			}

			// Move to previous item
			l.offsetIdx--
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
func (l *List) Render() string {
	if len(l.items) == 0 {
		return ""
	}

	var lines []string
	currentIdx := l.offsetIdx
	currentOffset := l.offsetLine

	linesNeeded := l.height

	for linesNeeded > 0 && currentIdx < len(l.items) {
		item := l.getItem(currentIdx)
		itemLines := strings.Split(item.content, "\n")
		itemHeight := len(itemLines)

		if currentOffset >= 0 && currentOffset < itemHeight {
			// Add visible content lines
			lines = append(lines, itemLines[currentOffset:]...)

			// Add gap if this is not the absolute last visual element (conceptually gaps are between items)
			// But in the loop we can just add it and trim later
			if l.gap > 0 {
				for i := 0; i < l.gap; i++ {
					lines = append(lines, "")
				}
			}
		} else {
			// offsetLine starts in the gap
			gapOffset := currentOffset - itemHeight
			gapRemaining := l.gap - gapOffset
			if gapRemaining > 0 {
				for range gapRemaining {
					lines = append(lines, "")
				}
			}
		}

		linesNeeded = l.height - len(lines)
		currentIdx++
		currentOffset = 0 // Reset offset for subsequent items
	}

	l.height = max(l.height, 0)

	if len(lines) > l.height {
		lines = lines[:l.height]
	}

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

// SetItems sets the items in the list.
func (l *List) SetItems(items ...Item) {
	l.setItems(true, items...)
}

// setItems sets the items in the list. If evict is true, it clears the
// rendered item cache.
func (l *List) setItems(evict bool, items ...Item) {
	l.items = items
	l.selectedIdx = min(l.selectedIdx, len(l.items)-1)
	l.offsetIdx = min(l.offsetIdx, len(l.items)-1)
	l.offsetLine = 0
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

	// Remove the item
	l.items = append(l.items[:idx], l.items[idx+1:]...)

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

// countLines counts the number of lines in a string.
func countLines(s string) int {
	if s == "" {
		return 1
	}
	return strings.Count(s, "\n") + 1
}
