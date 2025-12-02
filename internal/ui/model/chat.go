package model

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/tui/components/anim"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/google/uuid"
)

// ChatAnimItem represents a chat animation item in the chat UI.
type ChatAnimItem struct {
	list.BaseFocusable
	anim *anim.Anim
}

var (
	_ list.Item      = (*ChatAnimItem)(nil)
	_ list.Focusable = (*ChatAnimItem)(nil)
)

// NewChatAnimItem creates a new instance of [ChatAnimItem].
func NewChatAnimItem(a *anim.Anim) *ChatAnimItem {
	m := new(ChatAnimItem)
	return m
}

// Init initializes the chat animation item.
func (c *ChatAnimItem) Init() tea.Cmd {
	return c.anim.Init()
}

// Step advances the animation by one step.
func (c *ChatAnimItem) Step() tea.Cmd {
	return c.anim.Step()
}

// SetLabel sets the label for the animation item.
func (c *ChatAnimItem) SetLabel(label string) {
	c.anim.SetLabel(label)
}

// Draw implements list.Item.
func (c *ChatAnimItem) Draw(scr uv.Screen, area uv.Rectangle) {
	styled := uv.NewStyledString(c.anim.View())
	styled.Draw(scr, area)
}

// Height implements list.Item.
func (c *ChatAnimItem) Height(int) int {
	return 1
}

// ID implements list.Item.
func (c *ChatAnimItem) ID() string {
	return "anim"
}

// ChatNoContentItem represents a chat item with no content.
type ChatNoContentItem struct {
	*list.StringItem
}

// NewChatNoContentItem creates a new instance of [ChatNoContentItem].
func NewChatNoContentItem(t *styles.Styles, id string) *ChatNoContentItem {
	c := new(ChatNoContentItem)
	c.StringItem = list.NewStringItem(id, "No message content").
		WithFocusStyles(&t.Chat.NoContentMessage, &t.Chat.NoContentMessage)
	return c
}

// ChatMessageItem represents a chat message item in the chat UI.
type ChatMessageItem struct {
	item list.Item
	msg  message.Message
}

var (
	_ list.Item          = (*ChatMessageItem)(nil)
	_ list.Focusable     = (*ChatMessageItem)(nil)
	_ list.Highlightable = (*ChatMessageItem)(nil)
)

// NewChatMessageItem creates a new instance of [ChatMessageItem].
func NewChatMessageItem(t *styles.Styles, msg message.Message) *ChatMessageItem {
	c := new(ChatMessageItem)

	switch msg.Role {
	case message.User:
		item := list.NewMarkdownItem(msg.ID, msg.Content().String()).
			WithFocusStyles(&t.Chat.UserMessageFocused, &t.Chat.UserMessageBlurred)
		item.SetHighlightStyle(list.LipglossStyleToCellStyler(t.TextSelection))
		// TODO: Add attachments
		c.item = item
	default:
		var thinkingContent string
		content := msg.Content().String()
		thinking := msg.IsThinking()
		finished := msg.IsFinished()
		finishedData := msg.FinishPart()
		reasoningContent := msg.ReasoningContent()
		reasoningThinking := strings.TrimSpace(reasoningContent.Thinking)

		if finished && content == "" && finishedData.Reason == message.FinishReasonError {
			tag := t.Chat.ErrorTag.Render("ERROR")
			title := t.Chat.ErrorTitle.Render(finishedData.Message)
			details := t.Chat.ErrorDetails.Render(finishedData.Details)
			errContent := fmt.Sprintf("%s %s\n\n%s", tag, title, details)

			item := list.NewStringItem(msg.ID, errContent).
				WithFocusStyles(&t.Chat.AssistantMessageFocused, &t.Chat.AssistantMessageBlurred)

			c.item = item

			return c
		}

		if thinking || reasoningThinking != "" {
			// TODO: animation item?
			// TODO: thinking item
			thinkingContent = reasoningThinking
		} else if finished && content == "" && finishedData.Reason == message.FinishReasonCanceled {
			content = "*Canceled*"
		}

		var parts []string
		if thinkingContent != "" {
			parts = append(parts, thinkingContent)
		}

		if content != "" {
			if len(parts) > 0 {
				parts = append(parts, "")
			}
			parts = append(parts, content)
		}

		item := list.NewMarkdownItem(msg.ID, strings.Join(parts, "\n")).
			WithFocusStyles(&t.Chat.AssistantMessageFocused, &t.Chat.AssistantMessageBlurred)
		item.SetHighlightStyle(list.LipglossStyleToCellStyler(t.TextSelection))

		c.item = item
	}

	return c
}

// Draw implements list.Item.
func (c *ChatMessageItem) Draw(scr uv.Screen, area uv.Rectangle) {
	c.item.Draw(scr, area)
}

// Height implements list.Item.
func (c *ChatMessageItem) Height(width int) int {
	return c.item.Height(width)
}

// ID implements list.Item.
func (c *ChatMessageItem) ID() string {
	return c.item.ID()
}

// Blur implements list.Focusable.
func (c *ChatMessageItem) Blur() {
	if blurable, ok := c.item.(list.Focusable); ok {
		blurable.Blur()
	}
}

// Focus implements list.Focusable.
func (c *ChatMessageItem) Focus() {
	if focusable, ok := c.item.(list.Focusable); ok {
		focusable.Focus()
	}
}

// IsFocused implements list.Focusable.
func (c *ChatMessageItem) IsFocused() bool {
	if focusable, ok := c.item.(list.Focusable); ok {
		return focusable.IsFocused()
	}
	return false
}

// GetHighlight implements list.Highlightable.
func (c *ChatMessageItem) GetHighlight() (startLine int, startCol int, endLine int, endCol int) {
	if highlightable, ok := c.item.(list.Highlightable); ok {
		return highlightable.GetHighlight()
	}
	return 0, 0, 0, 0
}

// SetHighlight implements list.Highlightable.
func (c *ChatMessageItem) SetHighlight(startLine int, startCol int, endLine int, endCol int) {
	if highlightable, ok := c.item.(list.Highlightable); ok {
		highlightable.SetHighlight(startLine, startCol, endLine, endCol)
	}
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
	l := list.New()
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
	m.list.Draw(scr, area)
}

// SetSize sets the size of the chat view port.
func (m *Chat) SetSize(width, height int) {
	m.list.SetSize(width, height)
}

// Len returns the number of items in the chat list.
func (m *Chat) Len() int {
	return m.list.Len()
}

// PrependItem prepends a new item to the chat list.
func (m *Chat) PrependItem(item list.Item) {
	m.list.PrependItem(item)
}

// AppendMessage appends a new message item to the chat list.
func (m *Chat) AppendMessage(msg message.Message) {
	if msg.ID == "" {
		m.AppendItem(NewChatNoContentItem(m.com.Styles, uuid.NewString()))
	} else {
		m.AppendItem(NewChatMessageItem(m.com.Styles, msg))
	}
}

// AppendItem appends a new item to the chat list.
func (m *Chat) AppendItem(item list.Item) {
	if m.Len() > 0 {
		// Always add a spacer between messages
		m.list.AppendItem(list.NewSpacerItem(uuid.NewString(), 1))
	}
	m.list.AppendItem(item)
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

// SetSelectedIndex sets the selected message index in the chat list.
func (m *Chat) SetSelectedIndex(index int) {
	m.list.SetSelectedIndex(index)
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
