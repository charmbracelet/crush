package chat

import (
	"fmt"
	"image"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

// this is the total width that is taken up by the border + padding
// we also cap the width so text is readable to the maxTextWidth(120)
const messageLeftPaddingTotal = 2

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
	list.Highlightable
	list.Focusable
	Identifiable
}

type highlightableMessageItem struct {
	startLine   int
	startCol    int
	endLine     int
	endCol      int
	highlighter list.Highlighter
}

// isHighlighted returns true if the item has a highlight range set.
func (h *highlightableMessageItem) isHighlighted() bool {
	return h.startLine != -1 || h.endLine != -1
}

// renderHighlighted highlights the content if necessary.
func (h *highlightableMessageItem) renderHighlighted(content string, width, height int) string {
	if !h.isHighlighted() {
		return content
	}
	area := image.Rect(0, 0, width, height)
	return list.Highlight(content, area, h.startLine, h.startCol, h.endLine, h.endCol, h.highlighter)
}

// Highlight implements MessageItem.
func (h *highlightableMessageItem) Highlight(startLine int, startCol int, endLine int, endCol int) {
	// Adjust columns for the style's left inset (border + padding) since we
	// highlight the content only.
	offset := messageLeftPaddingTotal
	h.startLine = startLine
	h.startCol = max(0, startCol-offset)
	h.endLine = endLine
	if endCol >= 0 {
		h.endCol = max(0, endCol-offset)
	} else {
		h.endCol = endCol
	}
}

type UserMessageItem struct {
	highlightableMessageItem
	message *message.Message
	sty     *styles.Styles
	focused bool

	// holds the last capped width so we can determine if we can reuse the cache
	lastCappedWidth int

	// holds the rendered item, excluding the focus border
	// this way we do not need to rerender the whole thing on focus change
	cachedRender string
	cachedHeight int
}

func NewUserMessageItem(sty *styles.Styles, message *message.Message) MessageItem {
	return &UserMessageItem{
		highlightableMessageItem: highlightableMessageItem{
			highlighter: list.ToHighlighter(sty.TextSelection),
		},
		message: message,
		sty:     sty,
		focused: false,
	}
}

// Render implements MessageItem.
func (m *UserMessageItem) Render(width int) string {
	// this is the total width that is taken up by the border + padding
	//  we also cap the width so text is readable to the maxTextWidth(120)
	const messageLeftPaddingTotal = 2
	cappedWidth := min(width-messageLeftPaddingTotal, maxTextWidth)

	style := m.sty.Chat.Message.UserBlurred
	if m.focused {
		style = m.sty.Chat.Message.UserFocused
	}

	content := m.cachedRender

	// check if we can reuse the cache
	if cappedWidth == m.lastCappedWidth && m.cachedRender != "" {
		return style.Render(m.renderHighlighted(content, cappedWidth, m.cachedHeight))
	}

	renderer := common.MarkdownRenderer(m.sty, cappedWidth)

	msgContent := strings.TrimSpace(m.message.Content().Text)
	result, err := renderer.Render(msgContent)
	if err != nil {
		content = msgContent
	} else {
		content = strings.TrimSuffix(result, "\n")
	}

	if len(m.message.BinaryContent()) > 0 {
		attachmentsStr := m.renderAttachments(cappedWidth)
		content = strings.Join([]string{content, "", attachmentsStr}, "\n")
	}
	m.lastCappedWidth = cappedWidth
	m.cachedHeight = lipgloss.Height(content)
	m.cachedRender = content

	return style.Render(m.renderHighlighted(content, cappedWidth, m.cachedHeight))
}

// SetFocused implements MessageItem.
func (m *UserMessageItem) SetFocused(focused bool) {
	m.focused = focused
}

// ID implements MessageItem.
func (m *UserMessageItem) ID() string {
	return m.message.ID
}

// renderAttachments renders attachments with wrapping if they exceed the width.
// TODO: change the styles here so they match the new design
func (m *UserMessageItem) renderAttachments(width int) string {
	const maxFilenameWidth = 10

	attachments := make([]string, len(m.message.BinaryContent()))
	for i, attachment := range m.message.BinaryContent() {
		filename := filepath.Base(attachment.Path)
		attachments[i] = m.sty.Chat.Message.Attachment.Render(fmt.Sprintf(
			" %s %s ",
			styles.DocumentIcon,
			ansi.Truncate(filename, maxFilenameWidth, "..."),
		))
	}

	// Wrap attachments into lines that fit within the width.
	var lines []string
	var currentLine []string
	currentWidth := 0

	for _, att := range attachments {
		attWidth := lipgloss.Width(att)
		sepWidth := 1
		if len(currentLine) == 0 {
			sepWidth = 0
		}

		if currentWidth+sepWidth+attWidth > width && len(currentLine) > 0 {
			lines = append(lines, strings.Join(currentLine, " "))
			currentLine = []string{att}
			currentWidth = attWidth
		} else {
			currentLine = append(currentLine, att)
			currentWidth += sepWidth + attWidth
		}
	}

	if len(currentLine) > 0 {
		lines = append(lines, strings.Join(currentLine, " "))
	}

	return strings.Join(lines, "\n")
}

// GetMessageItems extracts [MessageItem]s from a [message.Message]. It returns
// all parts of the message as [MessageItem]s.
//
// For assistant messages with tool calls, pass a toolResults map to link results.
// Use BuildToolResultMap to create this map from all messages in a session.
func GetMessageItems(sty *styles.Styles, msg *message.Message, toolResults map[string]message.ToolResult) []MessageItem {
	switch msg.Role {
	case message.User:
		return []MessageItem{NewUserMessageItem(sty, msg)}
	}
	return []MessageItem{}
}

// BuildToolResultMap creates a map of tool call IDs to their results from a list of messages.
// Tool result messages (role == message.Tool) contain the results that should be linked
// to tool calls in assistant messages.
func BuildToolResultMap(messages []*message.Message) map[string]message.ToolResult {
	resultMap := make(map[string]message.ToolResult)
	for _, msg := range messages {
		if msg.Role == message.Tool {
			for _, result := range msg.ToolResults() {
				if result.ToolCallID != "" {
					resultMap[result.ToolCallID] = result
				}
			}
		}
	}
	return resultMap
}
