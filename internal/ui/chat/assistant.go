package chat

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/common/anim"
	"github.com/charmbracelet/crush/internal/ui/list"
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

	spinning         bool
	anim             *anim.Anim
	hasToolCalls     bool
	isSummaryMessage bool

	thinkingStartedAt  int64
	thinkingFinishedAt int64
}

// NewAssistantMessage creates a new assistant message item.
func NewAssistantMessage(id, content, thinking string, finished bool, finish message.Finish, hasToolCalls, isSummaryMessage bool, thinkingStartedAt, thinkingFinishedAt int64, sty *styles.Styles) *AssistantMessageItem {
	m := &AssistantMessageItem{
		id:                 id,
		content:            content,
		thinking:           thinking,
		finished:           finished,
		finish:             finish,
		hasToolCalls:       hasToolCalls,
		isSummaryMessage:   isSummaryMessage,
		thinkingStartedAt:  thinkingStartedAt,
		thinkingFinishedAt: thinkingFinishedAt,
		sty:                sty,
	}

	m.anim = anim.New(anim.Settings{
		Size:        15,
		GradColorA:  sty.Primary,
		GradColorB:  sty.Secondary,
		LabelColor:  sty.FgBase,
		CycleColors: true,
	})
	m.spinning = m.shouldSpin()

	return m
}

// shouldSpin returns true if the message should show loading animation.
func (m *AssistantMessageItem) shouldSpin() bool {
	if m.finished {
		return false
	}
	if strings.TrimSpace(m.content) != "" {
		return false
	}
	if m.hasToolCalls {
		return false
	}
	return true
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
	if m.spinning && m.thinking == "" {
		if m.isSummaryMessage {
			m.anim.SetLabel("Summarizing")
		}
		return m.anim.View()
	}

	cappedWidth := min(width, maxTextWidth)
	content := strings.TrimSpace(m.content)
	thinking := strings.TrimSpace(m.thinking)

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
	if thinking != "" {
		parts = append(parts, m.renderThinking(thinking, cappedWidth))
	}

	if content != "" {
		if len(parts) > 0 {
			parts = append(parts, "")
		}
		parts = append(parts, m.renderMarkdown(content, cappedWidth))
	}

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// Update implements list.Updatable for handling animation updates.
func (m *AssistantMessageItem) Update(msg tea.Msg) (list.Item, tea.Cmd) {
	switch msg.(type) {
	case anim.StepMsg:
		m.spinning = m.shouldSpin()
		if !m.spinning {
			return m, nil
		}
		updatedAnim, cmd := m.anim.Update(msg)
		m.anim = updatedAnim
		if cmd != nil {
			return m, cmd
		}
	}

	return m, nil
}

// InitAnimation initializes and starts the animation.
func (m *AssistantMessageItem) InitAnimation() tea.Cmd {
	m.spinning = m.shouldSpin()
	return m.anim.Init()
}

// SetContent updates the assistant message with new content.
func (m *AssistantMessageItem) SetContent(content, thinking string, finished bool, finish *message.Finish, hasToolCalls, isSummaryMessage bool, reasoning message.ReasoningContent) {
	m.content = content
	m.thinking = thinking
	m.finished = finished
	if finish != nil {
		m.finish = *finish
	}
	m.hasToolCalls = hasToolCalls
	m.isSummaryMessage = isSummaryMessage
	m.thinkingStartedAt = reasoning.StartedAt
	m.thinkingFinishedAt = reasoning.FinishedAt
	m.spinning = m.shouldSpin()
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

// renderThinking renders the thinking/reasoning content with footer.
func (m *AssistantMessageItem) renderThinking(thinking string, width int) string {
	renderer := common.PlainMarkdownRenderer(m.sty, width-2)
	rendered, err := renderer.Render(thinking)
	if err != nil {
		rendered = thinking
	}
	rendered = strings.TrimSpace(rendered)

	lines := strings.Split(rendered, "\n")
	totalLines := len(lines)

	isTruncated := totalLines > maxCollapsedThinkingHeight
	if !m.thinkingExpanded && isTruncated {
		lines = lines[totalLines-maxCollapsedThinkingHeight:]
	}

	if !m.thinkingExpanded && isTruncated {
		hint := m.sty.Chat.Message.ThinkingTruncationHint.Render(
			fmt.Sprintf("… (%d lines hidden) [click or space to expand]", totalLines-maxCollapsedThinkingHeight),
		)
		lines = append([]string{hint}, lines...)
	}

	thinkingStyle := m.sty.Chat.Message.ThinkingBox.Width(width)
	result := thinkingStyle.Render(strings.Join(lines, "\n"))
	m.thinkingBoxHeight = lipgloss.Height(result)

	var footer string
	if m.thinkingStartedAt > 0 {
		if m.thinkingFinishedAt > 0 {
			duration := time.Duration(m.thinkingFinishedAt-m.thinkingStartedAt) * time.Second
			if duration.String() != "0s" {
				footer = m.sty.Chat.Message.ThinkingFooterTitle.Render("Thought for ") +
					m.sty.Chat.Message.ThinkingFooterDuration.Render(duration.String())
			}
		} else if m.finish.Reason == message.FinishReasonCanceled {
			footer = m.sty.Chat.Message.ThinkingFooterCancelled.Render("Canceled")
		} else {
			m.anim.SetLabel("Thinking")
			footer = m.anim.View()
		}
	}

	if footer != "" {
		result += "\n\n" + footer
	}

	return result
}

// HandleMouseClick implements list.MouseClickable.
func (m *AssistantMessageItem) HandleMouseClick(btn ansi.MouseButton, x, y int) bool {
	if btn != ansi.MouseLeft {
		return false
	}

	if m.thinking != "" && y < m.thinkingBoxHeight {
		m.thinkingExpanded = !m.thinkingExpanded
		return true
	}

	return false
}

// HandleKeyPress implements list.KeyPressable.
func (m *AssistantMessageItem) HandleKeyPress(msg tea.KeyPressMsg) bool {
	if m.thinking == "" {
		return false
	}

	if key.Matches(msg, key.NewBinding(key.WithKeys("space"))) {
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
