package list

import (
	"strings"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/ultraviolet/screen"
	"github.com/charmbracelet/x/exp/ordered"
)

// List is a scrollable list component that implements uv.Drawable.
// It efficiently manages a large number of items by caching rendered content
// in a master buffer and extracting only the visible viewport when drawn.
type List struct {
	// Configuration
	width, height int

	// Data
	items []Item

	// Focus & Selection
	focused     bool
	selectedIdx int // Currently selected item index (-1 if none)

	// Master buffer containing ALL rendered items
	masterBuffer *uv.ScreenBuffer
	totalHeight  int

	// Item positioning in master buffer
	itemPositions []itemPosition

	// Viewport state
	offset int // Scroll offset in lines from top

	// Mouse state
	mouseDown     bool
	mouseDownItem int // Item index where mouse was pressed
	mouseDownX    int // X position in item content (character offset)
	mouseDownY    int // Y position in item (line offset)
	mouseDragItem int // Current item index being dragged over
	mouseDragX    int // Current X in item content
	mouseDragY    int // Current Y in item

	// Dirty tracking
	dirty      bool
	dirtyItems map[int]bool
}

type itemPosition struct {
	startLine int
	height    int
}

// New creates a new list with the given items.
func New(items ...Item) *List {
	l := &List{
		items:         items,
		itemPositions: make([]itemPosition, len(items)),
		dirtyItems:    make(map[int]bool),
		selectedIdx:   -1,
		mouseDownItem: -1,
		mouseDragItem: -1,
	}

	l.dirty = true
	return l
}

// ensureBuilt ensures the master buffer is built.
// This is called by methods that need itemPositions or totalHeight.
func (l *List) ensureBuilt() {
	if l.width <= 0 || l.height <= 0 {
		return
	}

	if l.dirty {
		l.rebuildMasterBuffer()
	} else if len(l.dirtyItems) > 0 {
		l.updateDirtyItems()
	}
}

// Draw implements uv.Drawable.
// Draws the visible viewport of the list to the given screen buffer.
func (l *List) Draw(scr uv.Screen, area uv.Rectangle) {
	if area.Dx() <= 0 || area.Dy() <= 0 {
		return
	}

	// Update internal dimensions if area size changed
	widthChanged := l.width != area.Dx()
	heightChanged := l.height != area.Dy()

	l.width = area.Dx()
	l.height = area.Dy()

	// Only width changes require rebuilding master buffer
	// Height changes only affect viewport clipping, not item rendering
	if widthChanged {
		l.dirty = true
	}

	// Height changes require clamping offset to new bounds
	if heightChanged {
		l.clampOffset()
	}

	if len(l.items) == 0 {
		screen.ClearArea(scr, area)
		return
	}

	// Ensure buffer is built
	l.ensureBuilt()

	// Draw visible portion to the target screen
	l.drawViewport(scr, area)
}

// Render renders the visible viewport to a string.
// This is a convenience method that creates a temporary screen buffer,
// draws to it, and returns the rendered string.
func (l *List) Render() string {
	if l.width <= 0 || l.height <= 0 {
		return ""
	}

	if len(l.items) == 0 {
		return ""
	}

	// Ensure buffer is built
	l.ensureBuilt()

	// Extract visible lines directly from master buffer
	return l.renderViewport()
}

// renderViewport renders the visible portion of the master buffer to a string.
func (l *List) renderViewport() string {
	if l.masterBuffer == nil {
		return ""
	}

	buf := l.masterBuffer.Buffer

	// Calculate visible region in master buffer
	srcStartY := l.offset
	srcEndY := l.offset + l.height

	// Clamp to actual buffer bounds
	if srcStartY >= len(buf.Lines) {
		// Beyond end of content, return empty lines
		emptyLine := strings.Repeat(" ", l.width)
		lines := make([]string, l.height)
		for i := range lines {
			lines[i] = emptyLine
		}
		return strings.Join(lines, "\n")
	}
	if srcEndY > len(buf.Lines) {
		srcEndY = len(buf.Lines)
	}

	// Build result with proper line handling
	lines := make([]string, l.height)
	lineIdx := 0

	// Render visible lines from buffer
	for y := srcStartY; y < srcEndY && lineIdx < l.height; y++ {
		lines[lineIdx] = buf.Lines[y].Render()
		lineIdx++
	}

	// Pad remaining lines with spaces to maintain viewport height
	emptyLine := strings.Repeat(" ", l.width)
	for ; lineIdx < l.height; lineIdx++ {
		lines[lineIdx] = emptyLine
	}

	return strings.Join(lines, "\n")
}

// drawViewport draws the visible portion from master buffer to target screen.
func (l *List) drawViewport(scr uv.Screen, area uv.Rectangle) {
	if l.masterBuffer == nil {
		screen.ClearArea(scr, area)
		return
	}

	buf := l.masterBuffer.Buffer

	// Calculate visible region in master buffer
	srcStartY := l.offset
	srcEndY := l.offset + area.Dy()

	// Clamp to actual buffer bounds
	if srcStartY >= buf.Height() {
		screen.ClearArea(scr, area)
		return
	}
	if srcEndY > buf.Height() {
		srcEndY = buf.Height()
	}

	// Copy visible lines to target screen
	destY := area.Min.Y
	for srcY := srcStartY; srcY < srcEndY && destY < area.Max.Y; srcY++ {
		line := buf.Line(srcY)
		destX := area.Min.X

		for x := 0; x < len(line) && x < area.Dx() && destX < area.Max.X; x++ {
			cell := line.At(x)
			scr.SetCell(destX, destY, cell)
			destX++
		}
		destY++
	}

	// Clear any remaining area if content is shorter than viewport
	if destY < area.Max.Y {
		clearArea := uv.Rect(area.Min.X, destY, area.Dx(), area.Max.Y-destY)
		screen.ClearArea(scr, clearArea)
	}
}

// rebuildMasterBuffer composes all items into the master buffer.
func (l *List) rebuildMasterBuffer() {
	if len(l.items) == 0 {
		l.totalHeight = 0
		l.dirty = false
		return
	}

	// Calculate total height
	l.totalHeight = l.calculateTotalHeight()

	// Create or resize master buffer
	if l.masterBuffer == nil || l.masterBuffer.Width() != l.width || l.masterBuffer.Height() != l.totalHeight {
		buf := uv.NewScreenBuffer(l.width, l.totalHeight)
		l.masterBuffer = &buf
	}

	// Clear buffer
	screen.Clear(l.masterBuffer)

	// Draw each item
	currentY := 0
	for i, item := range l.items {
		itemHeight := item.Height(l.width)

		// Draw item to master buffer
		area := uv.Rect(0, currentY, l.width, itemHeight)
		item.Draw(l.masterBuffer, area)

		// Store position
		l.itemPositions[i] = itemPosition{
			startLine: currentY,
			height:    itemHeight,
		}

		// Advance position
		currentY += itemHeight
	}

	l.dirty = false
	l.dirtyItems = make(map[int]bool)
}

// updateDirtyItems efficiently updates only changed items using slice operations.
func (l *List) updateDirtyItems() {
	if len(l.dirtyItems) == 0 {
		return
	}

	// Check if all dirty items have unchanged heights
	allSameHeight := true
	for idx := range l.dirtyItems {
		item := l.items[idx]
		pos := l.itemPositions[idx]
		newHeight := item.Height(l.width)
		if newHeight != pos.height {
			allSameHeight = false
			break
		}
	}

	// Optimization: If all dirty items have unchanged heights, re-render in place
	if allSameHeight {
		buf := l.masterBuffer.Buffer
		for idx := range l.dirtyItems {
			item := l.items[idx]
			pos := l.itemPositions[idx]

			// Clear the item's area
			for y := pos.startLine; y < pos.startLine+pos.height && y < len(buf.Lines); y++ {
				buf.Lines[y] = uv.NewLine(l.width)
			}

			// Re-render item
			area := uv.Rect(0, pos.startLine, l.width, pos.height)
			item.Draw(l.masterBuffer, area)
		}

		l.dirtyItems = make(map[int]bool)
		return
	}

	// Height changed - full rebuild
	l.dirty = true
	l.dirtyItems = make(map[int]bool)
	l.rebuildMasterBuffer()
}

// updatePositionsBelow updates the startLine for all items below the given index.
func (l *List) updatePositionsBelow(fromIdx int, delta int) {
	for i := fromIdx + 1; i < len(l.items); i++ {
		pos := l.itemPositions[i]
		pos.startLine += delta
		l.itemPositions[i] = pos
	}
}

// calculateTotalHeight calculates the total height of all items plus gaps.
func (l *List) calculateTotalHeight() int {
	if len(l.items) == 0 {
		return 0
	}

	total := 0
	for _, item := range l.items {
		total += item.Height(l.width)
	}
	return total
}

// SetSize updates the viewport size.
func (l *List) SetSize(width, height int) {
	widthChanged := l.width != width
	heightChanged := l.height != height

	l.width = width
	l.height = height

	// Width changes require full rebuild (items may reflow)
	if widthChanged {
		l.dirty = true
	}

	// Height changes require clamping offset to new bounds
	if heightChanged {
		l.clampOffset()
	}
}

// Height returns the current viewport height.
func (l *List) Height() int {
	return l.height
}

// Width returns the current viewport width.
func (l *List) Width() int {
	return l.width
}

// GetSize returns the current viewport size.
func (l *List) GetSize() (int, int) {
	return l.width, l.height
}

// Len returns the number of items in the list.
func (l *List) Len() int {
	return len(l.items)
}

// SetItems replaces all items in the list.
func (l *List) SetItems(items []Item) {
	l.items = items
	l.itemPositions = make([]itemPosition, len(items))
	l.dirty = true
}

// Items returns all items in the list.
func (l *List) Items() []Item {
	return l.items
}

// AppendItem adds an item to the end of the list. Returns true if successful.
func (l *List) AppendItem(item Item) bool {
	l.items = append(l.items, item)
	l.itemPositions = append(l.itemPositions, itemPosition{})

	// If buffer not built yet, mark dirty for full rebuild
	if l.masterBuffer == nil || l.width <= 0 {
		l.dirty = true
		return true
	}

	// Process any pending dirty items before modifying buffer structure
	if len(l.dirtyItems) > 0 {
		l.updateDirtyItems()
	}

	// Efficient append: insert lines at end of buffer
	itemHeight := item.Height(l.width)
	startLine := l.totalHeight

	// Expand buffer
	newLines := make([]uv.Line, itemHeight)
	for i := range newLines {
		newLines[i] = uv.NewLine(l.width)
	}
	l.masterBuffer.Buffer.Lines = append(l.masterBuffer.Buffer.Lines, newLines...)

	// Draw new item
	area := uv.Rect(0, startLine, l.width, itemHeight)
	item.Draw(l.masterBuffer, area)

	// Update tracking
	l.itemPositions[len(l.items)-1] = itemPosition{
		startLine: startLine,
		height:    itemHeight,
	}
	l.totalHeight += itemHeight

	return true
}

// PrependItem adds an item to the beginning of the list. Returns true if
// successful.
func (l *List) PrependItem(item Item) bool {
	l.items = append([]Item{item}, l.items...)
	l.itemPositions = append([]itemPosition{{}}, l.itemPositions...)
	if l.selectedIdx >= 0 {
		l.selectedIdx++
	}

	// If buffer not built yet, mark dirty for full rebuild
	if l.masterBuffer == nil || l.width <= 0 {
		l.dirty = true
		return true
	}

	// Process any pending dirty items before modifying buffer structure
	if len(l.dirtyItems) > 0 {
		l.updateDirtyItems()
	}

	// Efficient prepend: insert lines at start of buffer
	itemHeight := item.Height(l.width)

	// Create new lines
	newLines := make([]uv.Line, itemHeight)
	for i := range newLines {
		newLines[i] = uv.NewLine(l.width)
	}

	// Insert at beginning
	buf := l.masterBuffer.Buffer
	buf.Lines = append(newLines, buf.Lines...)

	// Draw new item
	area := uv.Rect(0, 0, l.width, itemHeight)
	item.Draw(l.masterBuffer, area)

	// Update all positions (shift everything down)
	for i := range l.itemPositions {
		pos := l.itemPositions[i]
		pos.startLine += itemHeight
		l.itemPositions[i] = pos
	}

	// Add position for new item at start
	l.itemPositions[0] = itemPosition{
		startLine: 0,
		height:    itemHeight,
	}

	l.totalHeight += itemHeight

	return true
}

// UpdateItem replaces an item with the same index. Returns true if successful.
func (l *List) UpdateItem(idx int, item Item) bool {
	if idx < 0 || idx >= len(l.items) {
		return false
	}
	l.items[idx] = item
	l.dirtyItems[idx] = true
	return true
}

// DeleteItem removes an item by index. Returns true if successful.
func (l *List) DeleteItem(idx int) bool {
	if idx < 0 || idx >= len(l.items) {
		return false
	}

	// Get position before deleting
	pos := l.itemPositions[idx]

	// Process any pending dirty items before modifying buffer structure
	if len(l.dirtyItems) > 0 {
		l.updateDirtyItems()
	}

	l.items = append(l.items[:idx], l.items[idx+1:]...)
	l.itemPositions = append(l.itemPositions[:idx], l.itemPositions[idx+1:]...)

	// Adjust selection
	if l.selectedIdx == idx {
		if idx > 0 {
			l.selectedIdx = idx - 1
		} else if len(l.items) > 0 {
			l.selectedIdx = 0
		} else {
			l.selectedIdx = -1
		}
	} else if l.selectedIdx > idx {
		l.selectedIdx--
	}

	// If buffer not built yet, mark dirty for full rebuild
	if l.masterBuffer == nil {
		l.dirty = true
		return true
	}

	// Efficient delete: remove lines from buffer
	deleteStart := pos.startLine
	deleteEnd := pos.startLine + pos.height
	buf := l.masterBuffer.Buffer

	if deleteEnd <= len(buf.Lines) {
		buf.Lines = append(buf.Lines[:deleteStart], buf.Lines[deleteEnd:]...)
		l.totalHeight -= pos.height
		l.updatePositionsBelow(idx-1, -pos.height)
	} else {
		// Position data corrupt, rebuild
		l.dirty = true
	}

	return true
}

// Focus focuses the list and the selected item (if focusable).
func (l *List) Focus() {
	l.focused = true
	l.focusSelectedItem()
}

// Blur blurs the list and the selected item (if focusable).
func (l *List) Blur() {
	l.focused = false
	l.blurSelectedItem()
}

// Focused returns whether the list is focused.
func (l *List) Focused() bool {
	return l.focused
}

// SetSelected sets the selected item by ID.
func (l *List) SetSelected(idx int) {
	if idx < 0 || idx >= len(l.items) {
		return
	}
	if l.selectedIdx == idx {
		return
	}

	prevIdx := l.selectedIdx
	l.selectedIdx = idx

	// Update focus states if list is focused
	if l.focused {
		if prevIdx >= 0 && prevIdx < len(l.items) {
			if f, ok := l.items[prevIdx].(Focusable); ok {
				f.Blur()
				l.dirtyItems[prevIdx] = true
			}
		}

		if f, ok := l.items[idx].(Focusable); ok {
			f.Focus()
			l.dirtyItems[idx] = true
		}
	}
}

// SelectFirst selects the first item in the list.
func (l *List) SelectFirst() {
	l.SetSelected(0)
}

// SelectLast selects the last item in the list.
func (l *List) SelectLast() {
	l.SetSelected(len(l.items) - 1)
}

// SelectNextWrap selects the next item in the list (wraps to beginning).
// When the list is focused, skips non-focusable items.
func (l *List) SelectNextWrap() {
	l.selectNext(true)
}

// SelectNext selects the next item in the list (no wrap).
// When the list is focused, skips non-focusable items.
func (l *List) SelectNext() {
	l.selectNext(false)
}

func (l *List) selectNext(wrap bool) {
	if len(l.items) == 0 {
		return
	}

	startIdx := l.selectedIdx
	for i := 0; i < len(l.items); i++ {
		var nextIdx int
		if wrap {
			nextIdx = (startIdx + 1 + i) % len(l.items)
		} else {
			nextIdx = startIdx + 1 + i
			if nextIdx >= len(l.items) {
				return
			}
		}

		// If list is focused and item is not focusable, skip it
		if l.focused {
			if _, ok := l.items[nextIdx].(Focusable); !ok {
				continue
			}
		}

		// Select and scroll to this item
		l.SetSelected(nextIdx)
		return
	}
}

// SelectPrevWrap selects the previous item in the list (wraps to end).
// When the list is focused, skips non-focusable items.
func (l *List) SelectPrevWrap() {
	l.selectPrev(true)
}

// SelectPrev selects the previous item in the list (no wrap).
// When the list is focused, skips non-focusable items.
func (l *List) SelectPrev() {
	l.selectPrev(false)
}

func (l *List) selectPrev(wrap bool) {
	if len(l.items) == 0 {
		return
	}

	startIdx := l.selectedIdx
	for i := 0; i < len(l.items); i++ {
		var prevIdx int
		if wrap {
			prevIdx = (startIdx - 1 - i + len(l.items)) % len(l.items)
		} else {
			prevIdx = startIdx - 1 - i
			if prevIdx < 0 {
				return
			}
		}

		// If list is focused and item is not focusable, skip it
		if l.focused {
			if _, ok := l.items[prevIdx].(Focusable); !ok {
				continue
			}
		}

		// Select and scroll to this item
		l.SetSelected(prevIdx)
		return
	}
}

// SelectedItem returns the currently selected item, or nil if none.
func (l *List) SelectedItem() Item {
	if l.selectedIdx < 0 || l.selectedIdx >= len(l.items) {
		return nil
	}
	return l.items[l.selectedIdx]
}

// SelectedIndex returns the index of the currently selected item, or -1 if none.
func (l *List) SelectedIndex() int {
	return l.selectedIdx
}

// AtBottom returns whether the viewport is scrolled to the bottom.
func (l *List) AtBottom() bool {
	l.ensureBuilt()
	return l.offset >= l.totalHeight-l.height
}

// AtTop returns whether the viewport is scrolled to the top.
func (l *List) AtTop() bool {
	return l.offset <= 0
}

// ScrollBy scrolls the viewport by the given number of lines.
// Positive values scroll down, negative scroll up.
func (l *List) ScrollBy(deltaLines int) {
	l.offset += deltaLines
	l.clampOffset()
}

// ScrollToTop scrolls to the top of the list.
func (l *List) ScrollToTop() {
	l.offset = 0
}

// ScrollToBottom scrolls to the bottom of the list.
func (l *List) ScrollToBottom() {
	l.ensureBuilt()
	if l.totalHeight > l.height {
		l.offset = l.totalHeight - l.height
	} else {
		l.offset = 0
	}
}

// ScrollToItem scrolls to make the item with the given ID visible.
func (l *List) ScrollToItem(idx int) {
	l.ensureBuilt()
	pos := l.itemPositions[idx]
	itemStart := pos.startLine
	itemEnd := pos.startLine + pos.height
	viewStart := l.offset
	viewEnd := l.offset + l.height

	// Check if item is already fully visible
	if itemStart >= viewStart && itemEnd <= viewEnd {
		return
	}

	// Scroll to show item
	if itemStart < viewStart {
		l.offset = itemStart
	} else if itemEnd > viewEnd {
		l.offset = itemEnd - l.height
	}

	l.clampOffset()
}

// ScrollToSelected scrolls to make the selected item visible.
func (l *List) ScrollToSelected() {
	if l.selectedIdx < 0 || l.selectedIdx >= len(l.items) {
		return
	}
	l.ScrollToItem(l.selectedIdx)
}

// Offset returns the current scroll offset.
func (l *List) Offset() int {
	return l.offset
}

// TotalHeight returns the total height of all items including gaps.
func (l *List) TotalHeight() int {
	return l.totalHeight
}

// SelectFirstInView selects the first item that is fully visible in the viewport.
func (l *List) SelectFirstInView() {
	l.ensureBuilt()

	viewportStart := l.offset
	viewportEnd := l.offset + l.height

	for i := range l.items {
		pos := l.itemPositions[i]

		// Check if item is fully within viewport bounds
		if pos.startLine >= viewportStart && (pos.startLine+pos.height) <= viewportEnd {
			l.SetSelected(i)
			return
		}
	}
}

// SelectLastInView selects the last item that is fully visible in the viewport.
func (l *List) SelectLastInView() {
	l.ensureBuilt()

	viewportStart := l.offset
	viewportEnd := l.offset + l.height

	for i := len(l.items) - 1; i >= 0; i-- {
		pos := l.itemPositions[i]

		// Check if item is fully within viewport bounds
		if pos.startLine >= viewportStart && (pos.startLine+pos.height) <= viewportEnd {
			l.SetSelected(i)
			return
		}
	}
}

// SelectedItemInView returns true if the selected item is currently visible in the viewport.
func (l *List) SelectedItemInView() bool {
	if l.selectedIdx < 0 || l.selectedIdx >= len(l.items) {
		return false
	}

	// Get selected item ID and position
	pos := l.itemPositions[l.selectedIdx]

	// Check if item is within viewport bounds
	viewportStart := l.offset
	viewportEnd := l.offset + l.height

	// Item is visible if any part of it overlaps with the viewport
	return pos.startLine < viewportEnd && (pos.startLine+pos.height) > viewportStart
}

// clampOffset ensures offset is within valid bounds.
func (l *List) clampOffset() {
	l.offset = ordered.Clamp(l.offset, 0, l.totalHeight-l.height)
}

// focusSelectedItem focuses the currently selected item if it's focusable.
func (l *List) focusSelectedItem() {
	if l.selectedIdx < 0 || l.selectedIdx >= len(l.items) {
		return
	}

	item := l.items[l.selectedIdx]
	if f, ok := item.(Focusable); ok {
		f.Focus()
		l.dirtyItems[l.selectedIdx] = true
	}
}

// blurSelectedItem blurs the currently selected item if it's focusable.
func (l *List) blurSelectedItem() {
	if l.selectedIdx < 0 || l.selectedIdx >= len(l.items) {
		return
	}

	item := l.items[l.selectedIdx]
	if f, ok := item.(Focusable); ok {
		f.Blur()
		l.dirtyItems[l.selectedIdx] = true
	}
}

// HandleMouseDown handles mouse button press events.
// x and y are viewport-relative coordinates (0,0 = top-left of visible area).
// Returns true if the event was handled.
func (l *List) HandleMouseDown(x, y int) bool {
	l.ensureBuilt()

	// Convert viewport y to master buffer y
	bufferY := y + l.offset

	// Find which item was clicked
	itemIdx, itemY := l.findItemAtPosition(bufferY)
	if itemIdx < 0 {
		return false
	}

	// Calculate x position within item content
	// For now, x is just the viewport x coordinate
	// Items can interpret this as character offset in their content

	l.mouseDown = true
	l.mouseDownItem = itemIdx
	l.mouseDownX = x
	l.mouseDownY = itemY
	l.mouseDragItem = itemIdx
	l.mouseDragX = x
	l.mouseDragY = itemY

	// Select the clicked item
	l.SetSelected(itemIdx)

	return true
}

// HandleMouseDrag handles mouse drag events during selection.
// x and y are viewport-relative coordinates.
// Returns true if the event was handled.
func (l *List) HandleMouseDrag(x, y int) bool {
	if !l.mouseDown {
		return false
	}

	l.ensureBuilt()

	// Convert viewport y to master buffer y
	bufferY := y + l.offset

	// Find which item we're dragging over
	itemIdx, itemY := l.findItemAtPosition(bufferY)
	if itemIdx < 0 {
		return false
	}

	l.mouseDragItem = itemIdx
	l.mouseDragX = x
	l.mouseDragY = itemY

	// Update highlight if item supports it
	l.updateHighlight()

	return true
}

// HandleMouseUp handles mouse button release events.
// Returns true if the event was handled.
func (l *List) HandleMouseUp(x, y int) bool {
	if !l.mouseDown {
		return false
	}

	l.mouseDown = false

	// Final highlight update
	l.updateHighlight()

	return true
}

// ClearHighlight clears any active text highlighting.
func (l *List) ClearHighlight() {
	for i, item := range l.items {
		if h, ok := item.(Highlightable); ok {
			h.SetHighlight(-1, -1, -1, -1)
			l.dirtyItems[i] = true
		}
	}
	l.mouseDownItem = -1
	l.mouseDragItem = -1
}

// findItemAtPosition finds the item at the given master buffer y coordinate.
// Returns the item index and the y offset within that item. It returns -1, -1
// if no item is found.
func (l *List) findItemAtPosition(bufferY int) (itemIdx int, itemY int) {
	if bufferY < 0 || bufferY >= l.totalHeight {
		return -1, -1
	}

	// Linear search through items to find which one contains this y
	// This could be optimized with binary search if needed
	for i := range l.items {
		pos := l.itemPositions[i]
		if bufferY >= pos.startLine && bufferY < pos.startLine+pos.height {
			return i, bufferY - pos.startLine
		}
	}

	return -1, -1
}

// updateHighlight updates the highlight range for highlightable items.
// Supports highlighting across multiple items and respects drag direction.
func (l *List) updateHighlight() {
	if l.mouseDownItem < 0 {
		return
	}

	// Get start and end item indices
	downItemIdx := l.mouseDownItem
	dragItemIdx := l.mouseDragItem

	// Determine selection direction
	draggingDown := dragItemIdx > downItemIdx ||
		(dragItemIdx == downItemIdx && l.mouseDragY > l.mouseDownY) ||
		(dragItemIdx == downItemIdx && l.mouseDragY == l.mouseDownY && l.mouseDragX >= l.mouseDownX)

	// Determine actual start and end based on direction
	var startItemIdx, endItemIdx int
	var startLine, startCol, endLine, endCol int

	if draggingDown {
		// Normal forward selection
		startItemIdx = downItemIdx
		endItemIdx = dragItemIdx
		startLine = l.mouseDownY
		startCol = l.mouseDownX
		endLine = l.mouseDragY
		endCol = l.mouseDragX
	} else {
		// Backward selection (dragging up)
		startItemIdx = dragItemIdx
		endItemIdx = downItemIdx
		startLine = l.mouseDragY
		startCol = l.mouseDragX
		endLine = l.mouseDownY
		endCol = l.mouseDownX
	}

	// Clear all highlights first
	for i, item := range l.items {
		if h, ok := item.(Highlightable); ok {
			h.SetHighlight(-1, -1, -1, -1)
			l.dirtyItems[i] = true
		}
	}

	// Highlight all items in range
	for idx := startItemIdx; idx <= endItemIdx; idx++ {
		item, ok := l.items[idx].(Highlightable)
		if !ok {
			continue
		}

		if idx == startItemIdx && idx == endItemIdx {
			// Single item selection
			item.SetHighlight(startLine, startCol, endLine, endCol)
		} else if idx == startItemIdx {
			// First item - from start position to end of item
			pos := l.itemPositions[idx]
			item.SetHighlight(startLine, startCol, pos.height-1, 9999) // 9999 = end of line
		} else if idx == endItemIdx {
			// Last item - from start of item to end position
			item.SetHighlight(0, 0, endLine, endCol)
		} else {
			// Middle item - fully highlighted
			pos := l.itemPositions[idx]
			item.SetHighlight(0, 0, pos.height-1, 9999)
		}

		l.dirtyItems[idx] = true
	}
}

// GetHighlightedText returns the plain text content of all highlighted regions
// across items, without any styling. Returns empty string if no highlights exist.
func (l *List) GetHighlightedText() string {
	l.ensureBuilt()

	if l.masterBuffer == nil {
		return ""
	}

	var result strings.Builder

	// Iterate through items to find highlighted ones
	for i, item := range l.items {
		h, ok := item.(Highlightable)
		if !ok {
			continue
		}

		startLine, startCol, endLine, endCol := h.GetHighlight()
		if startLine < 0 {
			continue
		}

		pos := l.itemPositions[i]

		// Extract text from highlighted region in master buffer
		for y := startLine; y <= endLine && y < pos.height; y++ {
			bufferY := pos.startLine + y
			if bufferY >= l.masterBuffer.Height() {
				break
			}

			line := l.masterBuffer.Line(bufferY)

			// Determine column range for this line
			colStart := 0
			if y == startLine {
				colStart = startCol
			}

			colEnd := len(line)
			if y == endLine {
				colEnd = min(endCol, len(line))
			}

			// Track last non-empty position to trim trailing spaces
			lastContentX := -1
			for x := colStart; x < colEnd && x < len(line); x++ {
				cell := line.At(x)
				if cell == nil || cell.IsZero() {
					continue
				}
				if cell.Content != "" && cell.Content != " " {
					lastContentX = x
				}
			}

			// Extract text from cells using String() method, up to last content
			endX := colEnd
			if lastContentX >= 0 {
				endX = lastContentX + 1
			}

			for x := colStart; x < endX && x < len(line); x++ {
				cell := line.At(x)
				if cell == nil || cell.IsZero() {
					continue
				}
				result.WriteString(cell.String())
			}

			// Add newline between lines (but not after the last line)
			if y < endLine && y < pos.height-1 {
				result.WriteRune('\n')
			}
		}

		// Add newline between items if there are more highlighted items
		if result.Len() > 0 {
			result.WriteRune('\n')
		}
	}

	// Trim trailing newline if present
	text := result.String()
	return strings.TrimSuffix(text, "\n")
}
