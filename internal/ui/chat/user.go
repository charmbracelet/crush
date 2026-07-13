package chat

import (
	"encoding/xml"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/attachments"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// skillInvocation represents the XML structure for a loaded skill.
type skillInvocation struct {
	Name         string `xml:"name"`
	Description  string `xml:"description"`
	Location     string `xml:"location"`
	Instructions string `xml:"instructions"`
}

// channelMessage represents the XML structure for a channel-originated
// message pushed by an MCP channel server.
type channelMessage struct {
	XMLName    xml.Name `xml:"channel"`
	Source     string   `xml:"source,attr"`
	Sender     string   `xml:"sender,attr"`
	SenderName string   `xml:"sender_name,attr"`
	Time       string   `xml:"time,attr"`
	Content    string   `xml:",chardata"`
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
	// cache hit
	if ok {
		return m.renderHighlighted(content, cappedWidth, height)
	}

	msgContent := strings.TrimSpace(m.message.Content().Text)

	// Check if this is a skill invocation (loaded_skill XML)
	if strings.HasPrefix(msgContent, "<loaded_skill>") {
		content = m.renderSkillInvocation(msgContent, cappedWidth)
		height = lipgloss.Height(content)
		m.setCachedRender(content, cappedWidth, height)
		return m.renderHighlighted(content, cappedWidth, height)
	}

	// Check if this is a channel-originated message.
	if strings.HasPrefix(msgContent, "<channel") {
		content = m.renderChannelMessage(msgContent, cappedWidth)
		height = lipgloss.Height(content)
		m.setCachedRender(content, cappedWidth, height)
		return m.renderHighlighted(content, cappedWidth, height)
	}

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

	height = lipgloss.Height(content)
	m.setCachedRender(content, cappedWidth, height)
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

// TODO(follow-up): suppress channel name for a session's default channel.

// renderChannelMessage parses a <channel source="..." ...>body</channel> element
// and renders a styled metadata header followed by the body as markdown.
func (m *UserMessageItem) renderChannelMessage(raw string, width int) string {
	var ch channelMessage
	if err := xml.Unmarshal([]byte(raw), &ch); err != nil {
		return m.fallbackRender(raw, width)
	}

	// Build the header line: channel source, sender, time.
	header := m.sty.Tool.ResourceName.Render(ch.Source)

	sender := ch.SenderName
	if sender == "" {
		sender = ch.Sender
	}
	if sender != "" {
		header += " " + m.sty.Tool.ResourceSize.Render("·") + " " + m.sty.Tool.ResourceSize.Render(sender)
	}

	ts := ch.Time
	if ts == "" && m.message.CreatedAt > 0 {
		ts = time.Unix(m.message.CreatedAt, 0).Format(time.TimeOnly)
	}
	if ts != "" {
		header += " " + m.sty.Tool.ResourceSize.Render("·") + " " + m.sty.Tool.ResourceSize.Render(ts)
	}

	headerLine := m.sty.Tool.Body.Render(header)

	// Render the body content as markdown.
	body := strings.TrimSpace(ch.Content)
	var bodyRendered string
	if body != "" {
		renderer := common.MarkdownRenderer(m.sty, width)
		mu := common.LockMarkdownRenderer(renderer)
		mu.Lock()
		result, err := renderer.Render(body)
		mu.Unlock()
		if err != nil {
			bodyRendered = body
		} else {
			bodyRendered = strings.TrimSuffix(result, "\n")
		}
	}

	if bodyRendered == "" {
		return headerLine
	}
	return headerLine + "\n" + bodyRendered
}

// fallbackRender renders text as plain markdown when XML parsing fails.
func (m *UserMessageItem) fallbackRender(content string, width int) string {
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
	return m.attachments.Render(attachments, false, width)
}

// HandleKeyEvent implements KeyEventHandler.
func (m *UserMessageItem) HandleKeyEvent(key tea.KeyMsg) (bool, tea.Cmd) {
	if k := key.String(); k == "c" || k == "y" {
		text := m.message.Content().Text
		return true, common.CopyToClipboard(text, "Message copied to clipboard")
	}
	return false, nil
}
