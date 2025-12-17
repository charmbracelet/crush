package chat

import (
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

// UserMessageItem represents a user message in the chat UI.
type UserMessageItem struct {
	*highlightableMessageItem
	*cachedMessageItem
	message *message.Message
	sty     *styles.Styles
	focused bool
}

// NewUserMessageItem creates a new UserMessageItem.
func NewUserMessageItem(sty *styles.Styles, message *message.Message) MessageItem {
	return &UserMessageItem{
		highlightableMessageItem: defaultHighlighter(sty),
		cachedMessageItem:        &cachedMessageItem{},
		message:                  message,
		sty:                      sty,
		focused:                  false,
	}
}

// Render implements MessageItem.
func (m *UserMessageItem) Render(width int) string {
	cappedWidth := cappedMessageWidth(width)

	style := m.sty.Chat.Message.UserBlurred
	if m.focused {
		style = m.sty.Chat.Message.UserFocused
	}

	content, height, ok := m.getCachedRender(cappedWidth)
	// cache hit
	if ok {
		return style.Render(m.renderHighlighted(content, cappedWidth, height))
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

	height = lipgloss.Height(content)
	m.setCachedRender(content, cappedWidth, height)
	return style.Render(m.renderHighlighted(content, cappedWidth, height))
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
			ansi.Truncate(filename, maxFilenameWidth, "…"),
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
