package list

import (
	"strings"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/ultraviolet/screen"
)

// List is a scrollable list component that implements uv.Drawable.
// It efficiently manages a large number of items by caching rendered content
// in a master buffer and extracting only the visible viewport when drawn.
type List struct {
	// Configuration
	width, height int

	// Data
	items    []Item
	indexMap map[string]int // ID -> index for fast lookup

	// Focus & Selection
	focused     bool
	selectedIdx int // Currently selected item index (-1 if none)

	// Master buffer containing ALL rendered items
	masterBuffer *uv.ScreenBuffer
	totalHeight  int

	// Item positioning in master buffer
	itemPositions map[string]itemPosition

	// Viewport state
	offset int // Scroll offset in lines from top

	// Dirty tracking
	dirty      bool
	dirtyItems map[string]bool
}

type itemPosition struct {
	startLine int
	height    int
}

// New creates a new list with the given items.
func New(items ...Item) *List {
	l := &List{
		items:         items,
		indexMap:      make(map[string]int, len(items)),
		itemPositions: make(map[string]itemPosition, len(items)),
		dirtyItems:    make(map[string]bool),
		selectedIdx:   -1,
	}

	// Build index map
	for i, item := range items {
		l.indexMap[item.ID()] = i
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
		return strings.Join(lines, "\r\n")
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

	return strings.Join(lines, "\r\n")
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
	if srcStartY >= len(buf.Lines) {
		screen.ClearArea(scr, area)
		return
	}
	if srcEndY > len(buf.Lines) {
		srcEndY = len(buf.Lines)
	}

	// Copy visible lines to target screen
	destY := area.Min.Y
	for srcY := srcStartY; srcY < srcEndY && destY < area.Max.Y; srcY++ {
		line := buf.Lines[srcY]
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
	for _, item := range l.items {
		itemHeight := item.Height(l.width)

		// Draw item to master buffer
		area := uv.Rect(0, currentY, l.width, itemHeight)
		item.Draw(l.masterBuffer, area)

		// Store position
		l.itemPositions[item.ID()] = itemPosition{
			startLine: currentY,
			height:    itemHeight,
		}

		// Advance position
		currentY += itemHeight
	}

	l.dirty = false
	l.dirtyItems = make(map[string]bool)
}

// updateDirtyItems efficiently updates only changed items using slice operations.
func (l *List) updateDirtyItems() {
	if len(l.dirtyItems) == 0 {
		return
	}

	// Check if all dirty items have unchanged heights
	allSameHeight := true
	for id := range l.dirtyItems {
		idx, ok := l.indexMap[id]
		if !ok {
			continue
		}

		item := l.items[idx]
		pos, ok := l.itemPositions[id]
		if !ok {
			l.dirty = true
			l.dirtyItems = make(map[string]bool)
			l.rebuildMasterBuffer()
			return
		}

		newHeight := item.Height(l.width)
		if newHeight != pos.height {
			allSameHeight = false
			break
		}
	}

	// Optimization: If all dirty items have unchanged heights, re-render in place
	if allSameHeight {
		buf := l.masterBuffer.Buffer
		for id := range l.dirtyItems {
			idx := l.indexMap[id]
			item := l.items[idx]
			pos := l.itemPositions[id]

			// Clear the item's area
			for y := pos.startLine; y < pos.startLine+pos.height && y < len(buf.Lines); y++ {
				buf.Lines[y] = uv.NewLine(l.width)
			}

			// Re-render item
			area := uv.Rect(0, pos.startLine, l.width, pos.height)
			item.Draw(l.masterBuffer, area)
		}

		l.dirtyItems = make(map[string]bool)
		return
	}

	// Height changed - full rebuild
	l.dirty = true
	l.dirtyItems = make(map[string]bool)
	l.rebuildMasterBuffer()
}

// updatePositionsBelow updates the startLine for all items below the given index.
func (l *List) updatePositionsBelow(fromIdx int, delta int) {
	for i := fromIdx + 1; i < len(l.items); i++ {
		item := l.items[i]
		pos := l.itemPositions[item.ID()]
		pos.startLine += delta
		l.itemPositions[item.ID()] = pos
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
	l.indexMap = make(map[string]int, len(items))
	l.itemPositions = make(map[string]itemPosition, len(items))

	for i, item := range items {
		l.indexMap[item.ID()] = i
	}

	l.dirty = true
}

// Items returns all items in the list.
func (l *List) Items() []Item {
	return l.items
}

// AppendItem adds an item to the end of the list.
func (l *List) AppendItem(item Item) {
	l.items = append(l.items, item)
	l.indexMap[item.ID()] = len(l.items) - 1

	// If buffer not built yet, mark dirty for full rebuild
	if l.masterBuffer == nil || l.width <= 0 {
		l.dirty = true
		return
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
	l.itemPositions[item.ID()] = itemPosition{
		startLine: startLine,
		height:    itemHeight,
	}
	l.totalHeight += itemHeight
}

// PrependItem adds an item to the beginning of the list.
func (l *List) PrependItem(item Item) {
	l.items = append([]Item{item}, l.items...)

	// Rebuild index map (all indices shifted)
	l.indexMap = make(map[string]int, len(l.items))
	for i, itm := range l.items {
		l.indexMap[itm.ID()] = i
	}

	if l.selectedIdx >= 0 {
		l.selectedIdx++
	}

	// If buffer not built yet, mark dirty for full rebuild
	if l.masterBuffer == nil || l.width <= 0 {
		l.dirty = true
		return
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
	for i := 1; i < len(l.items); i++ {
		itm := l.items[i]
		if pos, ok := l.itemPositions[itm.ID()]; ok {
			pos.startLine += itemHeight
			l.itemPositions[itm.ID()] = pos
		}
	}

	// Add position for new item
	l.itemPositions[item.ID()] = itemPosition{
		startLine: 0,
		height:    itemHeight,
	}
	l.totalHeight += itemHeight
}

// UpdateItem replaces an item with the same ID.
func (l *List) UpdateItem(id string, item Item) {
	idx, ok := l.indexMap[id]
	if !ok {
		return
	}

	l.items[idx] = item
	l.dirtyItems[id] = true
}

// DeleteItem removes an item by ID.
func (l *List) DeleteItem(id string) {
	idx, ok := l.indexMap[id]
	if !ok {
		return
	}

	// Get position before deleting
	pos, hasPos := l.itemPositions[id]

	// Process any pending dirty items before modifying buffer structure
	if len(l.dirtyItems) > 0 {
		l.updateDirtyItems()
	}

	l.items = append(l.items[:idx], l.items[idx+1:]...)
	delete(l.indexMap, id)
	delete(l.itemPositions, id)

	// Rebuild index map for items after deleted one
	for i := idx; i < len(l.items); i++ {
		l.indexMap[l.items[i].ID()] = i
	}

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
	if l.masterBuffer == nil || !hasPos {
		l.dirty = true
		return
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

// IsFocused returns whether the list is focused.
func (l *List) IsFocused() bool {
	return l.focused
}

// SetSelected sets the selected item by ID.
func (l *List) SetSelected(id string) {
	idx, ok := l.indexMap[id]
	if !ok {
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
				l.dirtyItems[l.items[prevIdx].ID()] = true
			}
		}

		if f, ok := l.items[idx].(Focusable); ok {
			f.Focus()
			l.dirtyItems[l.items[idx].ID()] = true
		}
	}
}

// SetSelectedIndex sets the selected item by index.
func (l *List) SetSelectedIndex(idx int) {
	if idx < 0 || idx >= len(l.items) {
		return
	}
	l.SetSelected(l.items[idx].ID())
}

// SelectNext selects the next item in the list (wraps to beginning).
// When the list is focused, skips non-focusable items.
func (l *List) SelectNext() {
	if len(l.items) == 0 {
		return
	}

	startIdx := l.selectedIdx
	for i := 0; i < len(l.items); i++ {
		nextIdx := (startIdx + 1 + i) % len(l.items)

		// If list is focused and item is not focusable, skip it
		if l.focused {
			if _, ok := l.items[nextIdx].(Focusable); !ok {
				continue
			}
		}

		// Select and scroll to this item
		l.SetSelected(l.items[nextIdx].ID())
		l.ScrollToSelected()
		return
	}
}

// SelectPrev selects the previous item in the list (wraps to end).
// When the list is focused, skips non-focusable items.
func (l *List) SelectPrev() {
	if len(l.items) == 0 {
		return
	}

	startIdx := l.selectedIdx
	for i := 0; i < len(l.items); i++ {
		prevIdx := (startIdx - 1 - i + len(l.items)) % len(l.items)

		// If list is focused and item is not focusable, skip it
		if l.focused {
			if _, ok := l.items[prevIdx].(Focusable); !ok {
				continue
			}
		}

		// Select and scroll to this item
		l.SetSelected(l.items[prevIdx].ID())
		l.ScrollToSelected()
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
func (l *List) ScrollToItem(id string) {
	l.ensureBuilt()
	pos, ok := l.itemPositions[id]
	if !ok {
		return
	}

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
	l.ScrollToItem(l.items[l.selectedIdx].ID())
}

// Offset returns the current scroll offset.
func (l *List) Offset() int {
	return l.offset
}

// TotalHeight returns the total height of all items including gaps.
func (l *List) TotalHeight() int {
	return l.totalHeight
}

// clampOffset ensures offset is within valid bounds.
func (l *List) clampOffset() {
	maxOffset := l.totalHeight - l.height
	if maxOffset < 0 {
		maxOffset = 0
	}

	if l.offset < 0 {
		l.offset = 0
	} else if l.offset > maxOffset {
		l.offset = maxOffset
	}
}

// focusSelectedItem focuses the currently selected item if it's focusable.
func (l *List) focusSelectedItem() {
	if l.selectedIdx < 0 || l.selectedIdx >= len(l.items) {
		return
	}

	item := l.items[l.selectedIdx]
	if f, ok := item.(Focusable); ok {
		f.Focus()
		l.dirtyItems[item.ID()] = true
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
		l.dirtyItems[item.ID()] = true
	}
}
