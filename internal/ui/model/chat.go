package model

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/anim"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// Chat represents the chat UI model that handles chat interactions and
// messages.
type Chat struct {
	com      *common.Common
	list     *list.List
	idInxMap map[string]int // Map of message IDs to their indices in the list

	// Animation visibility optimization: track animations paused due to items
	// being scrolled out of view. When items become visible again, their
	// animations are restarted.
	pausedAnimations map[string]struct{}

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
	c := &Chat{
		com:              com,
		idInxMap:         make(map[string]int),
		pausedAnimations: make(map[string]struct{}),
	}
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

// SetMessages sets the chat messages to the provided list of message items.
func (m *Chat) SetMessages(msgs ...chat.MessageItem) {
	m.idInxMap = make(map[string]int)
	m.pausedAnimations = make(map[string]struct{})

	items := make([]list.Item, len(msgs))
	for i, msg := range msgs {
		m.idInxMap[msg.ID()] = i
		// Register nested tool IDs for tools that contain nested tools.
		if container, ok := msg.(chat.NestedToolContainer); ok {
			for _, nested := range container.NestedTools() {
				m.idInxMap[nested.ID()] = i
			}
		}
		items[i] = msg
	}
	m.list.SetItems(items...)
	m.list.ScrollToBottom()
}

// AppendMessages appends a new message item to the chat list.
func (m *Chat) AppendMessages(msgs ...chat.MessageItem) {
	items := make([]list.Item, len(msgs))
	indexOffset := m.list.Len()
	for i, msg := range msgs {
		m.idInxMap[msg.ID()] = indexOffset + i
		// Register nested tool IDs for tools that contain nested tools.
		if container, ok := msg.(chat.NestedToolContainer); ok {
			for _, nested := range container.NestedTools() {
				m.idInxMap[nested.ID()] = indexOffset + i
			}
		}
		items[i] = msg
	}
	m.list.AppendItems(items...)
}

// UpdateNestedToolIDs updates the ID map for nested tools within a container.
// Call this after modifying nested tools to ensure animations work correctly.
func (m *Chat) UpdateNestedToolIDs(containerID string) {
	idx, ok := m.idInxMap[containerID]
	if !ok {
		return
	}

	item, ok := m.list.ItemAt(idx).(chat.MessageItem)
	if !ok {
		return
	}

	container, ok := item.(chat.NestedToolContainer)
	if !ok {
		return
	}

	// Register all nested tool IDs to point to the container's index.
	for _, nested := range container.NestedTools() {
		m.idInxMap[nested.ID()] = idx
	}
}

// Animate animates items in the chat list. Only propagates animation messages
// to visible items to save CPU. When items are not visible, their animation ID
// is tracked so it can be restarted when they become visible again.
func (m *Chat) Animate(msg anim.StepMsg) tea.Cmd {
	idx, ok := m.idInxMap[msg.ID]
	if !ok {
		return nil
	}

	animatable, ok := m.list.ItemAt(idx).(chat.Animatable)
	if !ok {
		return nil
	}

	// Check if item is currently visible.
	startIdx, endIdx := m.list.VisibleItemIndices()
	isVisible := idx >= startIdx && idx <= endIdx

	if !isVisible {
		// Item not visible - pause animation by not propagating.
		// Track it so we can restart when it becomes visible.
		m.pausedAnimations[msg.ID] = struct{}{}
		return nil
	}

	// Item is visible - remove from paused set and animate.
	delete(m.pausedAnimations, msg.ID)
	return animatable.Animate(msg)
}

// RestartPausedVisibleAnimations restarts animations for items that were paused
// due to being scrolled out of view but are now visible again.
func (m *Chat) RestartPausedVisibleAnimations() tea.Cmd {
	if len(m.pausedAnimations) == 0 {
		return nil
	}

	startIdx, endIdx := m.list.VisibleItemIndices()
	var cmds []tea.Cmd

	for id := range m.pausedAnimations {
		idx, ok := m.idInxMap[id]
		if !ok {
			// Item no longer exists.
			delete(m.pausedAnimations, id)
			continue
		}

		if idx >= startIdx && idx <= endIdx {
			// Item is now visible - restart its animation.
			if animatable, ok := m.list.ItemAt(idx).(chat.Animatable); ok {
				if cmd := animatable.StartAnimation(); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
			delete(m.pausedAnimations, id)
		}
	}

	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// Focus sets the focus state of the chat component.
func (m *Chat) Focus() {
	m.list.Focus()
}

// Blur removes the focus state from the chat component.
func (m *Chat) Blur() {
	m.list.Blur()
}

// ScrollToTopAndAnimate scrolls the chat view to the top and returns a command to restart
// any paused animations that are now visible.
func (m *Chat) ScrollToTopAndAnimate() tea.Cmd {
	m.list.ScrollToTop()
	return m.RestartPausedVisibleAnimations()
}

// ScrollToBottomAndAnimate scrolls the chat view to the bottom and returns a command to
// restart any paused animations that are now visible.
func (m *Chat) ScrollToBottomAndAnimate() tea.Cmd {
	m.list.ScrollToBottom()
	return m.RestartPausedVisibleAnimations()
}

// ScrollByAndAnimate scrolls the chat view by the given number of line deltas and returns
// a command to restart any paused animations that are now visible.
func (m *Chat) ScrollByAndAnimate(lines int) tea.Cmd {
	m.list.ScrollBy(lines)
	return m.RestartPausedVisibleAnimations()
}

// ScrollToSelectedAndAnimate scrolls the chat view to the selected item and returns a
// command to restart any paused animations that are now visible.
func (m *Chat) ScrollToSelectedAndAnimate() tea.Cmd {
	m.list.ScrollToSelected()
	return m.RestartPausedVisibleAnimations()
}

// SelectedItemInView returns whether the selected item is currently in view.
func (m *Chat) SelectedItemInView() bool {
	return m.list.SelectedItemInView()
}

func (m *Chat) isSelectable(index int) bool {
	item := m.list.ItemAt(index)
	if item == nil {
		return false
	}
	_, ok := item.(list.Focusable)
	return ok
}

// SetSelected sets the selected message index in the chat list.
func (m *Chat) SetSelected(index int) {
	m.list.SetSelected(index)
	if index < 0 || index >= m.list.Len() {
		return
	}
	for {
		if m.isSelectable(m.list.Selected()) {
			return
		}
		if m.list.SelectNext() {
			continue
		}
		// If we're at the end and the last item isn't selectable, walk backwards
		// to find the nearest selectable item.
		for {
			if !m.list.SelectPrev() {
				return
			}
			if m.isSelectable(m.list.Selected()) {
				return
			}
		}
	}
}

// SelectPrev selects the previous message in the chat list.
func (m *Chat) SelectPrev() {
	for {
		if !m.list.SelectPrev() {
			return
		}
		if m.isSelectable(m.list.Selected()) {
			return
		}
	}
}

// SelectNext selects the next message in the chat list.
func (m *Chat) SelectNext() {
	for {
		if !m.list.SelectNext() {
			return
		}
		if m.isSelectable(m.list.Selected()) {
			return
		}
	}
}

// SelectFirst selects the first message in the chat list.
func (m *Chat) SelectFirst() {
	if !m.list.SelectFirst() {
		return
	}
	if m.isSelectable(m.list.Selected()) {
		return
	}
	for {
		if !m.list.SelectNext() {
			return
		}
		if m.isSelectable(m.list.Selected()) {
			return
		}
	}
}

// SelectLast selects the last message in the chat list.
func (m *Chat) SelectLast() {
	if !m.list.SelectLast() {
		return
	}
	if m.isSelectable(m.list.Selected()) {
		return
	}
	for {
		if !m.list.SelectPrev() {
			return
		}
		if m.isSelectable(m.list.Selected()) {
			return
		}
	}
}

// SelectFirstInView selects the first message currently in view.
func (m *Chat) SelectFirstInView() {
	startIdx, endIdx := m.list.VisibleItemIndices()
	for i := startIdx; i <= endIdx; i++ {
		if m.isSelectable(i) {
			m.list.SetSelected(i)
			return
		}
	}
}

// SelectLastInView selects the last message currently in view.
func (m *Chat) SelectLastInView() {
	startIdx, endIdx := m.list.VisibleItemIndices()
	for i := endIdx; i >= startIdx; i-- {
		if m.isSelectable(i) {
			m.list.SetSelected(i)
			return
		}
	}
}

// ClearMessages removes all messages from the chat list.
func (m *Chat) ClearMessages() {
	m.idInxMap = make(map[string]int)
	m.pausedAnimations = make(map[string]struct{})
	m.list.SetItems()
	m.ClearMouse()
}

// RemoveMessage removes a message from the chat list by its ID.
func (m *Chat) RemoveMessage(id string) {
	idx, ok := m.idInxMap[id]
	if !ok {
		return
	}

	// Remove from list
	m.list.RemoveItem(idx)

	// Remove from index map
	delete(m.idInxMap, id)

	// Rebuild index map for all items after the removed one
	for i := idx; i < m.list.Len(); i++ {
		if item, ok := m.list.ItemAt(i).(chat.MessageItem); ok {
			m.idInxMap[item.ID()] = i
		}
	}

	// Clean up any paused animations for this message
	delete(m.pausedAnimations, id)
}

// MessageItem returns the message item with the given ID, or nil if not found.
func (m *Chat) MessageItem(id string) chat.MessageItem {
	idx, ok := m.idInxMap[id]
	if !ok {
		return nil
	}
	item, ok := m.list.ItemAt(idx).(chat.MessageItem)
	if !ok {
		return nil
	}
	return item
}

// ToggleExpandedSelectedItem expands the selected message item if it is expandable.
func (m *Chat) ToggleExpandedSelectedItem() {
	if expandable, ok := m.list.SelectedItem().(chat.Expandable); ok {
		expandable.ToggleExpanded()
	}
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
	if !m.isSelectable(itemIdx) {
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
