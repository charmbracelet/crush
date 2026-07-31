package chat

import (
	"encoding/xml"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/attachments"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

// skillInvocation represents the XML structure for a loaded skill.
type skillInvocation struct {
	Name         string `xml:"name"`
	Description  string `xml:"description"`
	Location     string `xml:"location"`
	Instructions string `xml:"instructions"`
}

// UserMessageItem represents a user message in the chat UI.
type UserMessageItem struct {
	*list.Versioned
	*highlightableMessageItem
	*cachedMessageItem
	*focusableMessageItem

	attachments *attachments.Renderer
	message     *message.Message
	sty         *styles.Styles

	// Click-to-copy icon geometry, recorded during RawRender so
	// HandleMouseClickCmd can hit-test without re-deriving the layout.
	copyIconRow      int
	copyIconColStart int
	copyIconColEnd   int
}

// NewUserMessageItem creates a new UserMessageItem.
func NewUserMessageItem(sty *styles.Styles, message *message.Message, attachments *attachments.Renderer) MessageItem {
	v := list.NewVersioned()
	return &UserMessageItem{
		Versioned:                v,
		highlightableMessageItem: defaultHighlighter(sty, v),
		cachedMessageItem:        &cachedMessageItem{},
		focusableMessageItem:     newFocusableMessageItem(v),
		attachments:              attachments,
		message:                  message,
		sty:                      sty,
	}
}

// Finished implements list.Item. User messages are immutable once
// submitted, so the entry is always safe to freeze.
func (m *UserMessageItem) Finished() bool {
	return true
}

// RawRender implements [MessageItem].
func (m *UserMessageItem) RawRender(width int) string {
	cappedWidth := cappedMessageWidth(width)

	content, height, ok := m.getCachedRender(cappedWidth)
	if !ok {
		msgContent := strings.TrimSpace(m.message.Content().Text)

		// Check if this is a skill invocation (loaded_skill XML)
		if strings.HasPrefix(msgContent, "<loaded_skill>") {
			content = m.renderSkillInvocation(msgContent, cappedWidth)
		} else {
			renderer := common.MarkdownRenderer(m.sty, cappedWidth)
			mu := common.LockMarkdownRenderer(renderer)

			mu.Lock()
			result, err := renderer.Render(msgContent)
			mu.Unlock()

			if err != nil {
				content = msgContent
			} else {
				content = strings.TrimSuffix(result, "\n")
			}

			if len(m.message.BinaryContent()) > 0 {
				attachmentsStr := m.renderAttachments(cappedWidth)
				if content == "" {
					content = attachmentsStr
				} else {
					content = strings.Join([]string{content, "", attachmentsStr}, "\n")
				}
			}
		}

		height = lipgloss.Height(content)
		m.setCachedRender(content, cappedWidth, height)
	}

	// Click-to-copy icon: right-aligned ⎘ appended to the last content
	// line, shown only while the message item is focused (selected).
	// The icon copies the raw Markdown source via HandleMouseClickCmd.
	m.copyIconRow = -1
	msgText := strings.TrimSpace(m.message.Content().Text)
	if msgText != "" && m.focused {
		icon := m.sty.Messages.AssistantCopyIcon.Render(assistantCopyIcon)
		iconWidth := lipgloss.Width(icon)
		m.copyIconRow = height - 1
		lastLine := content
		if idx := strings.LastIndex(content, "\n"); idx >= 0 {
			lastLine = content[idx+1:]
		}
		m.copyIconColStart = lipgloss.Width(lastLine)
		m.copyIconColEnd = m.copyIconColStart + iconWidth
		content += icon
		height = lipgloss.Height(content)
	}

	return m.renderHighlighted(content, cappedWidth, height)
}

// renderSkillInvocation renders a loaded_skill XML as a special UI element.
func (m *UserMessageItem) renderSkillInvocation(content string, width int) string {
	var skill skillInvocation
	if err := xml.Unmarshal([]byte(content), &skill); err != nil {
		// If parsing fails, just render as markdown
		renderer := common.MarkdownRenderer(m.sty, width)
		mu := common.LockMarkdownRenderer(renderer)

		mu.Lock()
		result, err := renderer.Render(content)
		mu.Unlock()

		if err != nil {
			return content
		}
		return strings.TrimSuffix(result, "\n")
	}

	return toolOutputSkillContent(m.sty, skill.Name, skill.Description)
}

// Render implements MessageItem.
func (m *UserMessageItem) Render(width int) string {
	// Bypass the prefix cache while a highlight range is active so
	// selection drags reflect immediately without invalidating the
	// cache. Highlight changes are intentionally applied "above" the
	// prefix cache.
	useCache := !m.isHighlighted()
	var key uint64
	if m.focused {
		key = 1
	}
	if useCache {
		if cached, ok := m.getCachedPrefixedRender(width, key); ok {
			return cached
		}
	}
	var prefix string
	if m.focused {
		prefix = m.sty.Messages.UserFocused.Render()
	} else {
		prefix = m.sty.Messages.UserBlurred.Render()
	}
	lines := strings.Split(m.RawRender(width), "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	out := strings.Join(lines, "\n")
	if useCache {
		m.setCachedPrefixedRender(out, width, key)
	}
	return out
}

// ID implements MessageItem.
func (m *UserMessageItem) ID() string {
	return m.message.ID
}

// renderAttachments renders attachments.
func (m *UserMessageItem) renderAttachments(width int) string {
	var attachments []message.Attachment
	for _, at := range m.message.BinaryContent() {
		attachments = append(attachments, message.Attachment{
			FileName: at.Path,
			MimeType: at.MIMEType,
		})
	}
	// This message is already posted, so the attachment can't be removed;
	// don't render the remove button.
	return m.attachments.Render(attachments, false, false, width)
}

// HandleMouseClick implements [list.MouseClickable]. Defers to
// HandleMouseClickCmd; the command return is discarded because the
// generic click dispatcher only checks the handled flag on this path.
func (m *UserMessageItem) HandleMouseClick(btn ansi.MouseButton, x, y int) bool {
	handled, _ := m.HandleMouseClickCmd(btn, x, y)
	return handled
}

// HandleMouseClickCmd implements [list.MouseClickCommandable]. A click on
// the click-to-copy icon returns the copy command, which suppresses the
// generic expansion toggle in the chat click dispatcher.
func (m *UserMessageItem) HandleMouseClickCmd(btn ansi.MouseButton, x, y int) (bool, tea.Cmd) {
	if btn != ansi.MouseLeft {
		return false, nil
	}
	if m.copyIconRow >= 0 && y == m.copyIconRow &&
		x >= m.copyIconColStart+MessageLeftPaddingTotal && x < m.copyIconColEnd+MessageLeftPaddingTotal {
		text := m.message.Content().Text
		return true, common.CopyToClipboard(text, "Message copied to clipboard")
	}
	return false, nil
}

// HandleKeyEvent implements KeyEventHandler.
func (m *UserMessageItem) HandleKeyEvent(key tea.KeyMsg) (bool, tea.Cmd) {
	if k := key.String(); k == "c" || k == "y" {
		text := m.message.Content().Text
		return true, common.CopyToClipboard(text, "Message copied to clipboard")
	}
	return false, nil
}
