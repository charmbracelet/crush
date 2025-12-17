package model

import (
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// Chat represents the chat UI model that handles chat interactions and
// messages.
type Chat struct {
	com  *common.Common
	list *list.List

	// Mouse state
	mouseDown     bool
	mouseDownItem int // Item index where mouse was pressed
	mouseDownX    int // X position in item content (character offset)
	mouseDownY    int // Y position in item (line offset)
	mouseDragItem int // Current item index being dragged over
	mouseDragX    int // Current X in item content
	mouseDragY    int // Current Y in item
}

// NewChat creates a new instance of [Chat] that handles chat interactions and
// messages.
func NewChat(com *common.Common) *Chat {
	c := &Chat{com: com}
	l := list.NewList()
	l.SetGap(1)
	l.RegisterRenderCallback(c.applyHighlightRange)
	c.list = l
	c.mouseDownItem = -1
	c.mouseDragItem = -1
	return c
}

// Height returns the height of the chat view port.
func (m *Chat) Height() int {
	return m.list.Height()
}

// Draw renders the chat UI component to the screen and the given area.
func (m *Chat) Draw(scr uv.Screen, area uv.Rectangle) {
	uv.NewStyledString(m.list.Render()).Draw(scr, area)
}

// SetSize sets the size of the chat view port.
func (m *Chat) SetSize(width, height int) {
	m.list.SetSize(width, height)
}

// Len returns the number of items in the chat list.
func (m *Chat) Len() int {
	return m.list.Len()
}

// PrependItems prepends new items to the chat list.
func (m *Chat) PrependItems(items ...list.Item) {
	m.list.PrependItems(items...)
	m.list.ScrollToIndex(0)
}

// SetMessages sets the chat messages to the provided list of message items.
func (m *Chat) SetMessages(msgs ...chat.MessageItem) {
	items := make([]list.Item, len(msgs))
	for i, msg := range msgs {
		items[i] = msg
	}
	m.list.SetItems(items...)
	m.list.ScrollToBottom()
}

// AppendMessages appends a new message item to the chat list.
func (m *Chat) AppendMessages(msgs ...chat.MessageItem) {
	items := make([]list.Item, len(msgs))
	for i, msg := range msgs {
		items[i] = msg
	}
	m.list.AppendItems(items...)
}

// AppendItems appends new items to the chat list.
func (m *Chat) AppendItems(items ...list.Item) {
	m.list.AppendItems(items...)
	m.list.ScrollToIndex(m.list.Len() - 1)
}

// Focus sets the focus state of the chat component.
func (m *Chat) Focus() {
	m.list.Focus()
}

// Blur removes the focus state from the chat component.
func (m *Chat) Blur() {
	m.list.Blur()
}

// ScrollToTop scrolls the chat view to the top.
func (m *Chat) ScrollToTop() {
	m.list.ScrollToTop()
}

// ScrollToBottom scrolls the chat view to the bottom.
func (m *Chat) ScrollToBottom() {
	m.list.ScrollToBottom()
}

// ScrollBy scrolls the chat view by the given number of line deltas.
func (m *Chat) ScrollBy(lines int) {
	m.list.ScrollBy(lines)
}

// ScrollToSelected scrolls the chat view to the selected item.
func (m *Chat) ScrollToSelected() {
	m.list.ScrollToSelected()
}

// SelectedItemInView returns whether the selected item is currently in view.
func (m *Chat) SelectedItemInView() bool {
	return m.list.SelectedItemInView()
}

// SetSelected sets the selected message index in the chat list.
func (m *Chat) SetSelected(index int) {
	m.list.SetSelected(index)
}

// SelectPrev selects the previous message in the chat list.
func (m *Chat) SelectPrev() {
	m.list.SelectPrev()
}

// SelectNext selects the next message in the chat list.
func (m *Chat) SelectNext() {
	m.list.SelectNext()
}

// SelectFirst selects the first message in the chat list.
func (m *Chat) SelectFirst() {
	m.list.SelectFirst()
}

// SelectLast selects the last message in the chat list.
func (m *Chat) SelectLast() {
	m.list.SelectLast()
}

// SelectFirstInView selects the first message currently in view.
func (m *Chat) SelectFirstInView() {
	m.list.SelectFirstInView()
}

// SelectLastInView selects the last message currently in view.
func (m *Chat) SelectLastInView() {
	m.list.SelectLastInView()
}

// HandleMouseDown handles mouse down events for the chat component.
func (m *Chat) HandleMouseDown(x, y int) bool {
	if m.list.Len() == 0 {
		return false
	}

	itemIdx, itemY := m.list.ItemIndexAtPosition(x, y)
	if itemIdx < 0 {
		return false
	}

	m.mouseDown = true
	m.mouseDownItem = itemIdx
	m.mouseDownX = x
	m.mouseDownY = itemY
	m.mouseDragItem = itemIdx
	m.mouseDragX = x
	m.mouseDragY = itemY

	// Select the item that was clicked
	m.list.SetSelected(itemIdx)

	if clickable, ok := m.list.SelectedItem().(list.MouseClickable); ok {
		return clickable.HandleMouseClick(ansi.MouseButton1, x, itemY)
	}

	return true
}

// HandleMouseUp handles mouse up events for the chat component.
func (m *Chat) HandleMouseUp(x, y int) bool {
	if !m.mouseDown {
		return false
	}

	// TODO: Handle the behavior when mouse is released after a drag selection
	// (e.g., copy selected text to clipboard)

	m.mouseDown = false
	return true
}

// HandleMouseDrag handles mouse drag events for the chat component.
func (m *Chat) HandleMouseDrag(x, y int) bool {
	if !m.mouseDown {
		return false
	}

	if m.list.Len() == 0 {
		return false
	}

	itemIdx, itemY := m.list.ItemIndexAtPosition(x, y)
	if itemIdx < 0 {
		return false
	}

	m.mouseDragItem = itemIdx
	m.mouseDragX = x
	m.mouseDragY = itemY

	return true
}

// ClearMouse clears the current mouse interaction state.
func (m *Chat) ClearMouse() {
	m.mouseDown = false
	m.mouseDownItem = -1
	m.mouseDragItem = -1
}

// applyHighlightRange applies the current highlight range to the chat items.
func (m *Chat) applyHighlightRange(idx, selectedIdx int, item list.Item) list.Item {
	if hi, ok := item.(list.Highlightable); ok {
		// Apply highlight
		startItemIdx, startLine, startCol, endItemIdx, endLine, endCol := m.getHighlightRange()
		sLine, sCol, eLine, eCol := -1, -1, -1, -1
		if idx >= startItemIdx && idx <= endItemIdx {
			if idx == startItemIdx && idx == endItemIdx {
				// Single item selection
				sLine = startLine
				sCol = startCol
				eLine = endLine
				eCol = endCol
			} else if idx == startItemIdx {
				// First item - from start position to end of item
				sLine = startLine
				sCol = startCol
				eLine = -1
				eCol = -1
			} else if idx == endItemIdx {
				// Last item - from start of item to end position
				sLine = 0
				sCol = 0
				eLine = endLine
				eCol = endCol
			} else {
				// Middle item - fully highlighted
				sLine = 0
				sCol = 0
				eLine = -1
				eCol = -1
			}
		}

		hi.Highlight(sLine, sCol, eLine, eCol)
		return hi.(list.Item)
	}

	return item
}

// getHighlightRange returns the current highlight range.
func (m *Chat) getHighlightRange() (startItemIdx, startLine, startCol, endItemIdx, endLine, endCol int) {
	if m.mouseDownItem < 0 {
		return -1, -1, -1, -1, -1, -1
	}

	downItemIdx := m.mouseDownItem
	dragItemIdx := m.mouseDragItem

	// Determine selection direction
	draggingDown := dragItemIdx > downItemIdx ||
		(dragItemIdx == downItemIdx && m.mouseDragY > m.mouseDownY) ||
		(dragItemIdx == downItemIdx && m.mouseDragY == m.mouseDownY && m.mouseDragX >= m.mouseDownX)

	if draggingDown {
		// Normal forward selection
		startItemIdx = downItemIdx
		startLine = m.mouseDownY
		startCol = m.mouseDownX
		endItemIdx = dragItemIdx
		endLine = m.mouseDragY
		endCol = m.mouseDragX
	} else {
		// Backward selection (dragging up)
		startItemIdx = dragItemIdx
		startLine = m.mouseDragY
		startCol = m.mouseDragX
		endItemIdx = downItemIdx
		endLine = m.mouseDownY
		endCol = m.mouseDownX
	}

	return startItemIdx, startLine, startCol, endItemIdx, endLine, endCol
}
