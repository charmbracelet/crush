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

type UserMessageItem struct {
	id          string
	content     string
	attachments []message.BinaryContent
	sty         *styles.Styles
}

func NewUserMessage(id, content string, attachments []message.BinaryContent, sty *styles.Styles) *UserMessageItem {
	return &UserMessageItem{
		id:          id,
		content:     content,
		attachments: attachments,
		sty:         sty,
	}
}

// ID implements Identifiable.
func (m *UserMessageItem) ID() string {
	return m.id
}

// FocusStyle returns the focus style.
func (m *UserMessageItem) FocusStyle() lipgloss.Style {
	return m.sty.Chat.Message.UserFocused
}

// BlurStyle returns the blur style.
func (m *UserMessageItem) BlurStyle() lipgloss.Style {
	return m.sty.Chat.Message.UserBlurred
}

// HighlightStyle returns the highlight style.
func (m *UserMessageItem) HighlightStyle() lipgloss.Style {
	return m.sty.TextSelection
}

// Render implements MessageItem.
func (m *UserMessageItem) Render(width int) string {
	cappedWidth := min(width, maxTextWidth)
	renderer := common.MarkdownRenderer(m.sty, cappedWidth)
	result, err := renderer.Render(m.content)
	var rendered string
	if err != nil {
		rendered = m.content
	} else {
		rendered = strings.TrimSuffix(result, "\n")
	}

	if len(m.attachments) > 0 {
		attachmentsStr := m.renderAttachments(cappedWidth)
		rendered = strings.Join([]string{rendered, "", attachmentsStr}, "\n")
	}
	return rendered
}

// renderAttachments renders attachments with wrapping if they exceed the width.
func (m *UserMessageItem) renderAttachments(width int) string {
	const maxFilenameWidth = 10

	attachments := make([]string, len(m.attachments))
	for i, attachment := range m.attachments {
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
