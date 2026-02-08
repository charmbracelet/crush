package chat

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/attachments"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/ui/util"
)

// copyModeState represents the state of copy selection mode.
type copyModeState int

const (
	copyModeInactive copyModeState = iota
	copyModeSelecting
)

const copyModeTimeout = 5 * time.Second

// UserMessageItem represents a user message in the chat UI.
type UserMessageItem struct {
	*highlightableMessageItem
	*cachedMessageItem
	*focusableMessageItem

	attachments   *attachments.Renderer
	message       *message.Message
	sty           *styles.Styles
	copyMode      copyModeState
	copyModeTimer time.Time
}

// NewUserMessageItem creates a new UserMessageItem.
func NewUserMessageItem(sty *styles.Styles, message *message.Message, attachmentsRenderer *attachments.Renderer) MessageItem {
	return &UserMessageItem{
		highlightableMessageItem: defaultHighlighter(sty),
		cachedMessageItem:        &cachedMessageItem{},
		focusableMessageItem:     &focusableMessageItem{},
		attachments:              attachmentsRenderer,
		message:                  message,
		sty:                      sty,
		copyMode:                 copyModeInactive,
	}
}

// RawRender implements [MessageItem].
func (m *UserMessageItem) RawRender(width int) string {
	cappedWidth := cappedMessageWidth(width)

	content, height, ok := m.getCachedRender(cappedWidth)
	// cache hit
	if ok {
		return m.renderHighlighted(content, cappedWidth, height)
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
		if content == "" {
			content = attachmentsStr
		} else {
			content = strings.Join([]string{content, "", attachmentsStr}, "\n")
		}
	}

	height = lipgloss.Height(content)
	m.setCachedRender(content, cappedWidth, height)
	return m.renderHighlighted(content, cappedWidth, height)
}

// Render implements MessageItem.
func (m *UserMessageItem) Render(width int) string {
	style := m.sty.Chat.Message.UserBlurred
	if m.focused {
		style = m.sty.Chat.Message.UserFocused
	}
	return style.Render(m.RawRender(width))
}

// ID implements MessageItem.
func (m *UserMessageItem) ID() string {
	return m.message.ID
}

// renderAttachments renders attachments.
func (m *UserMessageItem) renderAttachments(width int) string {
	var atts []message.Attachment
	for _, at := range m.message.BinaryContent() {
		atts = append(atts, message.Attachment{
			FileName: at.Path,
			MimeType: at.MIMEType,
		})
	}
	return m.attachments.RenderWithMode(atts, false, m.copyMode == copyModeSelecting, width)
}

// HandleKeyEvent implements KeyEventHandler.
func (m *UserMessageItem) HandleKeyEvent(keyMsg tea.KeyMsg) (bool, tea.Cmd) {
	k := keyMsg.String()

	// Check if copy mode has timed out
	if m.copyMode == copyModeSelecting && time.Since(m.copyModeTimer) > copyModeTimeout {
		m.copyMode = copyModeInactive
		m.clearCache()
	}

	// Handle copy mode selection
	if m.copyMode == copyModeSelecting {
		switch k {
		case "esc":
			m.copyMode = copyModeInactive
			m.clearCache()
			return true, nil
		case "enter", "c", "y":
			// Copy message text (default behavior)
			m.copyMode = copyModeInactive
			m.clearCache()
			text := m.message.Content().Text
			return true, common.CopyToClipboard(text, "Message text copied to clipboard")
		case "a":
			// Copy all text attachment contents
			m.copyMode = copyModeInactive
			m.clearCache()
			return m.copyAllTextAttachments()
		default:
			// Check for digit keys (0-9)
			if len(k) == 1 && k[0] >= '0' && k[0] <= '9' {
				idx := int(k[0] - '0')
				binaryContent := m.message.BinaryContent()
				if idx < len(binaryContent) {
					m.copyMode = copyModeInactive
					m.clearCache()
					return m.copyAttachmentAtIndex(idx)
				}
			}
			// Invalid key, stay in copy mode but refresh timer
			m.copyModeTimer = time.Now()
			return true, util.ReportInfo("Press 0-9 for attachment, Enter for message text, Esc to cancel")
		}
	}

	// Initiate copy mode
	if k == "c" || k == "y" {
		binaryContent := m.message.BinaryContent()

		// If no attachments, copy message text directly
		if len(binaryContent) == 0 {
			text := m.message.Content().Text
			return true, common.CopyToClipboard(text, "Message copied to clipboard")
		}

		// If only one text attachment, copy it directly (shortcut)
		if len(binaryContent) == 1 && strings.HasPrefix(binaryContent[0].MIMEType, "text/") {
			return m.copyAttachmentAtIndex(0)
		}

		// Enter copy selection mode
		m.copyMode = copyModeSelecting
		m.copyModeTimer = time.Now()
		m.clearCache() // Clear cache to show attachment indices
		return true, util.ReportInfo("Press 0-9 to copy attachment, Enter for message text, A for all text, Esc to cancel")
	}

	return false, nil
}

// copyAttachmentAtIndex copies the attachment at the given index.
func (m *UserMessageItem) copyAttachmentAtIndex(idx int) (bool, tea.Cmd) {
	binaryContent := m.message.BinaryContent()
	if idx >= len(binaryContent) {
		return false, nil
	}

	att := binaryContent[idx]
	if strings.HasPrefix(att.MIMEType, "text/") && len(att.Data) > 0 {
		return true, common.CopyToClipboard(string(att.Data), "Attachment content copied to clipboard")
	}
	// For binary attachments, copy the path
	return true, common.CopyToClipboard(att.Path, "Attachment path copied to clipboard")
}

// copyAllTextAttachments copies all text attachment contents.
func (m *UserMessageItem) copyAllTextAttachments() (bool, tea.Cmd) {
	binaryContent := m.message.BinaryContent()
	var textContents []string

	for _, bc := range binaryContent {
		if strings.HasPrefix(bc.MIMEType, "text/") && len(bc.Data) > 0 {
			textContents = append(textContents, string(bc.Data))
		}
	}

	if len(textContents) > 0 {
		return true, common.CopyToClipboard(strings.Join(textContents, "\n\n"), "All attachment contents copied to clipboard")
	}
	// No text attachments found, copy all paths
	var paths []string
	for _, bc := range binaryContent {
		paths = append(paths, bc.Path)
	}
	return true, common.CopyToClipboard(strings.Join(paths, "\n"), "Attachment paths copied to clipboard")
}
