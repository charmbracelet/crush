// Package chat provides the chat UI components for displaying and managing
// conversation messages between users and assistants.
package chat

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	uv "github.com/charmbracelet/ultraviolet"
)

// maxTextWidth is the maximum width text messages can be
const maxTextWidth = 120

// Identifiable is an interface for items that can provide a unique identifier.
type Identifiable interface {
	ID() string
}

// MessageItem represents a [message.Message] item that can be displayed in the
// UI and be part of a [list.List] identifiable by a unique ID.
type MessageItem interface {
	list.Item
	Identifiable
}

// Animatable is implemented by items that support animation initialization.
type Animatable interface {
	InitAnimation() tea.Cmd
}

// ToolItem is implemented by tool items that support mutable updates.
type ToolItem interface {
	SetResult(result message.ToolResult)
	SetCancelled()
	UpdateCall(call message.ToolCall)
	SetNestedCalls(calls []ToolCallContext)
	Context() *ToolCallContext
}

// Chat represents the chat UI model that handles chat interactions and
// messages.
type Chat struct {
	com  *common.Common
	list *list.List
}

// NewChat creates a new instance of [Chat] that handles chat interactions and
// messages.
func NewChat(com *common.Common) *Chat {
	l := list.NewList()
	l.SetGap(1)
	return &Chat{
		com:  com,
		list: l,
	}
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
func (m *Chat) SetMessages(msgs ...MessageItem) {
	items := make([]list.Item, len(msgs))
	for i, msg := range msgs {
		items[i] = msg
	}
	m.list.SetItems(items...)
	m.list.ScrollToBottom()
}

// AppendMessages appends a new message item to the chat list.
func (m *Chat) AppendMessages(msgs ...MessageItem) {
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

// UpdateMessage updates an existing message by ID. Returns true if the message
// was found and updated.
func (m *Chat) UpdateMessage(id string, msg MessageItem) bool {
	for i := 0; i < m.list.Len(); i++ {
		item := m.list.GetItemAt(i)
		if identifiable, ok := item.(Identifiable); ok && identifiable.ID() == id {
			return m.list.UpdateItemAt(i, msg)
		}
	}
	return false
}

// GetMessage returns the message with the given ID. Returns nil if not found.
func (m *Chat) GetMessage(id string) MessageItem {
	for i := 0; i < m.list.Len(); i++ {
		item := m.list.GetItemAt(i)
		if identifiable, ok := item.(Identifiable); ok && identifiable.ID() == id {
			if msg, ok := item.(MessageItem); ok {
				return msg
			}
		}
	}
	return nil
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
func (m *Chat) HandleMouseDown(x, y int) {
	m.list.HandleMouseDown(x, y)
}

// HandleMouseUp handles mouse up events for the chat component.
func (m *Chat) HandleMouseUp(x, y int) {
	m.list.HandleMouseUp(x, y)
}

// HandleMouseDrag handles mouse drag events for the chat component.
func (m *Chat) HandleMouseDrag(x, y int) {
	m.list.HandleMouseDrag(x, y)
}

// HandleKeyPress handles key press events for the currently selected item.
func (m *Chat) HandleKeyPress(msg tea.KeyPressMsg) bool {
	return m.list.HandleKeyPress(msg)
}

// UpdateItems propagates a message to all items that support updates (e.g.,
// for animations). Returns commands from updated items.
func (m *Chat) UpdateItems(msg tea.Msg) tea.Cmd {
	return m.list.UpdateItems(msg)
}

// ToolItemUpdater is implemented by tool items that support mutable updates.
type ToolItemUpdater interface {
	SetResult(result message.ToolResult)
	SetCancelled()
	UpdateCall(call message.ToolCall)
	SetNestedCalls(calls []ToolCallContext)
	Context() *ToolCallContext
}

// GetToolItem returns the tool item with the given ID, or nil if not found.
func (m *Chat) GetToolItem(id string) ToolItem {
	for i := 0; i < m.list.Len(); i++ {
		item := m.list.GetItemAt(i)
		if identifiable, ok := item.(Identifiable); ok && identifiable.ID() == id {
			if toolItem, ok := item.(ToolItem); ok {
				return toolItem
			}
		}
	}
	return nil
}

// InvalidateItem invalidates the render cache for the item with the given ID.
// Use after mutating an item via ToolItem methods.
func (m *Chat) InvalidateItem(id string) {
	for i := 0; i < m.list.Len(); i++ {
		item := m.list.GetItemAt(i)
		if identifiable, ok := item.(Identifiable); ok && identifiable.ID() == id {
			m.list.InvalidateItemAt(i)
			return
		}
	}
}

// DeleteMessage removes a message by ID. Returns true if found and deleted.
func (m *Chat) DeleteMessage(id string) bool {
	for i := m.list.Len() - 1; i >= 0; i-- {
		item := m.list.GetItemAt(i)
		if identifiable, ok := item.(Identifiable); ok && identifiable.ID() == id {
			return m.list.DeleteItemAt(i)
		}
	}
	return false
}
