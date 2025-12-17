package chat

import (
	"image"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
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

// SendMsg represents a message to send a chat message.
type SendMsg struct {
	Text        string
	Attachments []message.Attachment
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

func defaultHighlighter(sty *styles.Styles) *highlightableMessageItem {
	return &highlightableMessageItem{
		startLine:   -1,
		startCol:    -1,
		endLine:     -1,
		endCol:      -1,
		highlighter: list.ToHighlighter(sty.TextSelection),
	}
}

// cachedMessageItem caches rendered message content to avoid re-rendering.
//
// This should be used by any message that can store a cahced version of its render. e.x user,assistant... and so on
//
// THOUGHT(kujtim): we should consider if its efficient to store the render for different widths
// the issue with that could be memory usage
type cachedMessageItem struct {
	// rendered is the cached rendered string
	rendered string
	// width and height are the dimensions of the cached render
	width  int
	height int
}

// getCachedRender returns the cached render if it exists for the given width.
func (c *cachedMessageItem) getCachedRender(width int) (string, int, bool) {
	if c.width == width && c.rendered != "" {
		return c.rendered, c.height, true
	}
	return "", 0, false
}

// setCachedRender sets the cached render.
func (c *cachedMessageItem) setCachedRender(rendered string, width, height int) {
	c.rendered = rendered
	c.width = width
	c.height = height
}

// cappedMessageWidth returns the maximum width for message content for readability.
func cappedMessageWidth(availableWidth int) int {
	return min(availableWidth-messageLeftPaddingTotal, maxTextWidth)
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
