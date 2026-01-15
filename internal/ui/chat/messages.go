// Package chat provides UI components for displaying and managing chat messages.
// It defines message item types that can be rendered in a list view, including
// support for highlighting, focusing, and caching rendered content.
package chat

import (
	"fmt"
	"image"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/anim"
	"github.com/charmbracelet/crush/internal/ui/attachments"
	"github.com/charmbracelet/crush/internal/ui/common"
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

// Animatable is an interface for items that support animation.
type Animatable interface {
	StartAnimation() tea.Cmd
	Animate(msg anim.StepMsg) tea.Cmd
}

// Expandable is an interface for items that can be expanded or collapsed.
type Expandable interface {
	ToggleExpanded()
}

// MessageItem represents a [message.Message] item that can be displayed in the
// UI and be part of a [list.List] identifiable by a unique ID.
type MessageItem interface {
	list.Item
	Identifiable
}

// HighlightableMessageItem is a message item that supports highlighting.
type HighlightableMessageItem interface {
	MessageItem
	list.Highlightable
}

// FocusableMessageItem is a message item that supports focus.
type FocusableMessageItem interface {
	MessageItem
	list.Focusable
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
// This should be used by any message that can store a cached version of its render. e.x user,assistant... and so on
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

// clearCache clears the cached render.
func (c *cachedMessageItem) clearCache() {
	c.rendered = ""
	c.width = 0
	c.height = 0
}

// focusableMessageItem is a base struct for message items that can be focused.
type focusableMessageItem struct {
	focused bool
}

// SetFocused implements MessageItem.
func (f *focusableMessageItem) SetFocused(focused bool) {
	f.focused = focused
}

// AssistantInfoID returns a stable ID for assistant info items.
func AssistantInfoID(messageID string) string {
	return fmt.Sprintf("%s:assistant-info", messageID)
}

// AssistantInfoItem renders model info and response time after assistant completes.
type AssistantInfoItem struct {
	*cachedMessageItem

	id                  string
	message             *message.Message
	sty                 *styles.Styles
	lastUserMessageTime time.Time
}

// NewAssistantInfoItem creates a new AssistantInfoItem.
func NewAssistantInfoItem(sty *styles.Styles, message *message.Message, lastUserMessageTime time.Time) MessageItem {
	return &AssistantInfoItem{
		cachedMessageItem:   &cachedMessageItem{},
		id:                  AssistantInfoID(message.ID),
		message:             message,
		sty:                 sty,
		lastUserMessageTime: lastUserMessageTime,
	}
}

// ID implements MessageItem.
func (a *AssistantInfoItem) ID() string {
	return a.id
}

// Render implements MessageItem.
func (a *AssistantInfoItem) Render(width int) string {
	innerWidth := max(0, width-messageLeftPaddingTotal)
	content, _, ok := a.getCachedRender(innerWidth)
	if !ok {
		content = a.renderContent(innerWidth)
		height := lipgloss.Height(content)
		a.setCachedRender(content, innerWidth, height)
	}

	return a.sty.Chat.Message.SectionHeader.Render(content)
}

func (a *AssistantInfoItem) renderContent(width int) string {
	finishData := a.message.FinishPart()
	if finishData == nil {
		return ""
	}
	finishTime := time.Unix(finishData.Time, 0)
	duration := finishTime.Sub(a.lastUserMessageTime)
	infoMsg := a.sty.Chat.Message.AssistantInfoDuration.Render(duration.String())
	icon := a.sty.Chat.Message.AssistantInfoIcon.Render(styles.ModelIcon)
	model := config.Get().GetModel(a.message.Provider, a.message.Model)
	if model == nil {
		model = &catwalk.Model{Name: "Unknown Model"}
	}
	modelFormatted := a.sty.Chat.Message.AssistantInfoModel.Render(model.Name)
	providerName := a.message.Provider
	if providerConfig, ok := config.Get().Providers.Get(a.message.Provider); ok {
		providerName = providerConfig.Name
	}
	provider := a.sty.Chat.Message.AssistantInfoProvider.Render(fmt.Sprintf("via %s", providerName))
	assistant := fmt.Sprintf("%s %s %s %s", icon, modelFormatted, provider, infoMsg)
	return common.Section(a.sty, assistant, width)
}

// cappedMessageWidth returns the maximum width for message content for readability.
func cappedMessageWidth(availableWidth int) int {
	return min(availableWidth-messageLeftPaddingTotal, maxTextWidth)
}

// ExtractMessageItems extracts [MessageItem]s from a [message.Message]. It
// returns all parts of the message as [MessageItem]s.
//
// For assistant messages with tool calls, pass a toolResults map to link results.
// Use BuildToolResultMap to create this map from all messages in a session.
func ExtractMessageItems(sty *styles.Styles, msg *message.Message, toolResults map[string]message.ToolResult) []MessageItem {
	switch msg.Role {
	case message.User:
		r := attachments.NewRenderer(
			sty.Attachments.Normal,
			sty.Attachments.Deleting,
			sty.Attachments.Image,
			sty.Attachments.Text,
		)
		return []MessageItem{NewUserMessageItem(sty, msg, r)}
	case message.Assistant:
		var items []MessageItem
		if ShouldRenderAssistantMessage(msg) {
			items = append(items, NewAssistantMessageItem(sty, msg))
		}
		for _, tc := range msg.ToolCalls() {
			var result *message.ToolResult
			if tr, ok := toolResults[tc.ID]; ok {
				result = &tr
			}
			items = append(items, NewToolMessageItem(
				sty,
				msg.ID,
				tc,
				result,
				msg.FinishReason() == message.FinishReasonCanceled,
			))
		}
		return items
	}
	return []MessageItem{}
}

// ShouldRenderAssistantMessage determines if an assistant message should be rendered
//
// In some cases the assistant message only has tools so we do not want to render an
// empty message.
func ShouldRenderAssistantMessage(msg *message.Message) bool {
	content := strings.TrimSpace(msg.Content().Text)
	thinking := strings.TrimSpace(msg.ReasoningContent().Thinking)
	isError := msg.FinishReason() == message.FinishReasonError
	isCancelled := msg.FinishReason() == message.FinishReasonCanceled
	hasToolCalls := len(msg.ToolCalls()) > 0
	return !hasToolCalls || content != "" || thinking != "" || msg.IsThinking() || isError || isCancelled
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
