package chat

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

const maxCollapsedThinkingHeight = 10

// AssistantMessageItem represents an assistant message that can be displayed
// in the chat UI.
type AssistantMessageItem struct {
	id                string
	content           string
	thinking          string
	finished          bool
	finish            message.Finish
	sty               *styles.Styles
	thinkingExpanded  bool
	thinkingBoxHeight int // Tracks the rendered thinking box height for click detection.
}

// NewAssistantMessage creates a new assistant message item.
func NewAssistantMessage(id, content, thinking string, finished bool, finish message.Finish, sty *styles.Styles) *AssistantMessageItem {
	return &AssistantMessageItem{
		id:       id,
		content:  content,
		thinking: thinking,
		finished: finished,
		finish:   finish,
		sty:      sty,
	}
}

// ID implements Identifiable.
func (m *AssistantMessageItem) ID() string {
	return m.id
}

// FocusStyle returns the focus style.
func (m *AssistantMessageItem) FocusStyle() lipgloss.Style {
	return m.sty.Chat.Message.AssistantFocused
}

// BlurStyle returns the blur style.
func (m *AssistantMessageItem) BlurStyle() lipgloss.Style {
	return m.sty.Chat.Message.AssistantBlurred
}

// HighlightStyle returns the highlight style.
func (m *AssistantMessageItem) HighlightStyle() lipgloss.Style {
	return m.sty.TextSelection
}

// Render implements list.Item.
func (m *AssistantMessageItem) Render(width int) string {
	cappedWidth := min(width, maxTextWidth)
	content := strings.TrimSpace(m.content)
	thinking := strings.TrimSpace(m.thinking)

	// Handle empty finished messages.
	if m.finished && content == "" {
		switch m.finish.Reason {
		case message.FinishReasonEndTurn:
			return ""
		case message.FinishReasonCanceled:
			return m.renderMarkdown("*Canceled*", cappedWidth)
		case message.FinishReasonError:
			return m.renderError(cappedWidth)
		}
	}

	var parts []string

	// Render thinking content if present.
	if thinking != "" {
		parts = append(parts, m.renderThinking(thinking, cappedWidth))
	}

	// Render main content.
	if content != "" {
		if len(parts) > 0 {
			parts = append(parts, "")
		}
		parts = append(parts, m.renderMarkdown(content, cappedWidth))
	}

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// renderMarkdown renders content as markdown.
func (m *AssistantMessageItem) renderMarkdown(content string, width int) string {
	renderer := common.MarkdownRenderer(m.sty, width)
	result, err := renderer.Render(content)
	if err != nil {
		return content
	}
	return strings.TrimSuffix(result, "\n")
}

// renderThinking renders the thinking/reasoning content.
func (m *AssistantMessageItem) renderThinking(thinking string, width int) string {
	renderer := common.PlainMarkdownRenderer(m.sty, width-2)
	rendered, err := renderer.Render(thinking)
	if err != nil {
		rendered = thinking
	}
	rendered = strings.TrimSpace(rendered)

	lines := strings.Split(rendered, "\n")
	totalLines := len(lines)

	// Collapse if not expanded and exceeds max height.
	isTruncated := totalLines > maxCollapsedThinkingHeight
	if !m.thinkingExpanded && isTruncated {
		lines = lines[totalLines-maxCollapsedThinkingHeight:]
	}

	// Add hint if truncated and not expanded.
	if !m.thinkingExpanded && isTruncated {
		hint := m.sty.Muted.Render(fmt.Sprintf("… (%d lines hidden) [click or space to expand]", totalLines-maxCollapsedThinkingHeight))
		lines = append([]string{hint}, lines...)
	}

	thinkingStyle := m.sty.Subtle.Background(m.sty.BgBaseLighter).Width(width)
	result := thinkingStyle.Render(strings.Join(lines, "\n"))

	// Track the rendered height for click detection.
	m.thinkingBoxHeight = lipgloss.Height(result)

	return result
}

// HandleMouseClick implements list.MouseClickable.
func (m *AssistantMessageItem) HandleMouseClick(btn ansi.MouseButton, x, y int) bool {
	// Only handle left clicks.
	if btn != ansi.MouseLeft {
		return false
	}

	// Check if click is within the thinking box area.
	if m.thinking != "" && y < m.thinkingBoxHeight {
		m.thinkingExpanded = !m.thinkingExpanded
		return true
	}

	return false
}

// HandleKeyPress implements list.KeyPressable.
func (m *AssistantMessageItem) HandleKeyPress(msg tea.KeyPressMsg) bool {
	// Only handle space key on thinking content.
	if m.thinking == "" {
		return false
	}

	if key.Matches(msg, key.NewBinding(key.WithKeys("space"))) {
		// Toggle thinking expansion.
		m.thinkingExpanded = !m.thinkingExpanded
		return true
	}

	return false
}

// renderError renders an error message.
func (m *AssistantMessageItem) renderError(width int) string {
	errTag := m.sty.Chat.Message.ErrorTag.Render("ERROR")
	truncated := ansi.Truncate(m.finish.Message, width-2-lipgloss.Width(errTag), "...")
	title := fmt.Sprintf("%s %s", errTag, m.sty.Chat.Message.ErrorTitle.Render(truncated))
	details := m.sty.Chat.Message.ErrorDetails.Width(width - 2).Render(m.finish.Details)
	return fmt.Sprintf("%s\n\n%s", title, details)
}
