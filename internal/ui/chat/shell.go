package chat

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/taigrr/crush/internal/message"
	"github.com/taigrr/crush/internal/ui/list"
	"github.com/taigrr/crush/internal/ui/styles"
)

// ShellMessageItem renders a bang-mode shell command with expandable output.
type ShellMessageItem struct {
	*list.Versioned
	*highlightableMessageItem
	*cachedMessageItem
	*focusableMessageItem

	message         *message.Message
	sty             *styles.Styles
	expandedContent bool
}

var (
	_ MessageItem = (*ShellMessageItem)(nil)
	_ Expandable  = (*ShellMessageItem)(nil)
)

// NewShellMessageItem creates a new ShellMessageItem.
func NewShellMessageItem(sty *styles.Styles, msg *message.Message) MessageItem {
	v := list.NewVersioned()
	return &ShellMessageItem{
		Versioned:                v,
		highlightableMessageItem: defaultHighlighter(sty, v),
		cachedMessageItem:        &cachedMessageItem{},
		focusableMessageItem:     newFocusableMessageItem(v),
		message:                  msg,
		sty:                      sty,
	}
}

// Finished implements list.Item.
func (m *ShellMessageItem) Finished() bool {
	return true
}

// ToggleExpanded implements Expandable.
func (m *ShellMessageItem) ToggleExpanded() bool {
	m.expandedContent = !m.expandedContent
	m.clearCache()
	m.Bump()
	return m.expandedContent
}

// RawRender implements [MessageItem].
func (m *ShellMessageItem) RawRender(width int) string {
	cappedWidth := cappedMessageWidth(width)

	content, height, ok := m.getCachedRender(cappedWidth)
	if ok {
		return m.renderHighlighted(content, cappedWidth, height)
	}

	text := strings.TrimSpace(m.message.Content().Text)
	command, output := parseShellMessage(text)

	header := toolHeader(m.sty, ToolStatusSuccess, "Shell", cappedWidth, false, command)

	if output == "" {
		content = header
	} else {
		bodyWidth := cappedWidth - toolBodyLeftPaddingTotal
		body := m.sty.Tool.Body.Render(toolOutputPlainContent(m.sty, output, bodyWidth, m.expandedContent))
		content = joinToolParts(header, body)
	}

	height = lipgloss.Height(content)
	m.setCachedRender(content, cappedWidth, height)
	return m.renderHighlighted(content, cappedWidth, height)
}

// Render implements MessageItem.
func (m *ShellMessageItem) Render(width int) string {
	return m.RawRender(width)
}

// parseShellMessage splits the stored "$ cmd\noutput" format.
func parseShellMessage(text string) (command, output string) {
	if !strings.HasPrefix(text, "$ ") {
		return text, ""
	}
	text = strings.TrimPrefix(text, "$ ")
	if cmd, rest, ok := strings.Cut(text, "\n"); ok {
		return cmd, rest
	}
	return text, ""
}

// ID implements Identifiable.
func (m *ShellMessageItem) ID() string {
	return fmt.Sprintf("shell-%s", m.message.ID)
}
