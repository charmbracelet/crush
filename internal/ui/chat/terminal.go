package chat

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

// TerminalOutputItem represents a locally executed terminal command output in the chat UI.
type TerminalOutputItem struct {
	*highlightableMessageItem
	*cachedMessageItem
	*focusableMessageItem

	id       string
	command  string
	stdout   string
	stderr   string
	exitCode int
	duration time.Duration
	hasError bool
	sty      *styles.Styles
}

// NewTerminalOutputItem creates a new TerminalOutputItem.
func NewTerminalOutputItem(
	id string,
	sty *styles.Styles,
	command string,
	stdout string,
	stderr string,
	exitCode int,
	duration time.Duration,
	hasError bool,
) MessageItem {
	return &TerminalOutputItem{
		highlightableMessageItem: defaultHighlighter(sty),
		cachedMessageItem:        &cachedMessageItem{},
		focusableMessageItem:     &focusableMessageItem{},
		id:                       id,
		command:                  command,
		stdout:                   stdout,
		stderr:                   stderr,
		exitCode:                 exitCode,
		duration:                 duration,
		hasError:                 hasError,
		sty:                      sty,
	}
}

// ID implements MessageItem.
func (m *TerminalOutputItem) ID() string {
	return m.id
}

// RawRender implements MessageItem.
func (m *TerminalOutputItem) RawRender(width int) string {
	cappedWidth := cappedMessageWidth(width)

	content, height, ok := m.getCachedRender(cappedWidth)
	if ok {
		return m.renderHighlighted(content, cappedWidth, height)
	}

	textWidth := cappedWidth - 4 // Account for border and padding

	// Header with command
	cmdIcon := lipgloss.NewStyle().
		Foreground(m.sty.BgSubtle).
		Background(m.sty.Yellow).
		Padding(0, 1).
		Bold(true).
		Render("$")

	cmdText := lipgloss.NewStyle().
		Foreground(m.sty.FgBase).
		Bold(true).
		Render(" " + m.command)

	header := lipgloss.JoinHorizontal(lipgloss.Left, cmdIcon, cmdText)

	// Combine stdout and stderr
	output := strings.TrimSpace(m.stdout)
	if m.stderr != "" {
		if output != "" {
			output += "\n"
		}
		output += strings.TrimSpace(m.stderr)
	}

	if output == "" {
		output = lipgloss.NewStyle().Foreground(m.sty.FgMuted).Render("(no output)")
	} else {
		// Truncate long output
		lines := strings.Split(output, "\n")
		const maxLines = 50
		if len(lines) > maxLines {
			truncatedCount := len(lines) - maxLines
			lines = lines[:maxLines]
			output = strings.Join(lines, "\n")
			output += fmt.Sprintf("\n... [%d more lines]", truncatedCount)
		}

		// Wrap long lines
		output = ansi.Wordwrap(output, textWidth, "")
	}

	outputStyle := lipgloss.NewStyle().
		Foreground(m.sty.FgMuted).
		Width(textWidth)

	var outputParts []string
	outputParts = append(outputParts, header, "", outputStyle.Render(output))

	// Footer with execution info
	var footerParts []string

	// Duration
	durationText := lipgloss.NewStyle().Foreground(m.sty.FgMuted).Render(m.duration.Round(time.Millisecond).String())
	footerParts = append(footerParts, durationText)

	// Exit code
	if m.exitCode != 0 {
		exitStyle := lipgloss.NewStyle().
			Foreground(m.sty.Red).
			Bold(true)
		footerParts = append(footerParts, exitStyle.Render(fmt.Sprintf("exit %d", m.exitCode)))
	}

	if len(footerParts) > 0 {
		outputParts = append(outputParts, "")
		footer := lipgloss.NewStyle().PaddingLeft(1).Render(strings.Join(footerParts, " • "))
		outputParts = append(outputParts, footer)
	}

	joined := lipgloss.JoinVertical(lipgloss.Left, outputParts...)

	// Container style
	borderColor := m.sty.Green
	if m.hasError {
		borderColor = m.sty.Red
	}
	if m.focused {
		borderColor = m.sty.Primary
	}

	containerStyle := lipgloss.NewStyle().
		PaddingLeft(1).
		BorderLeft(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(borderColor)

	content = containerStyle.Render(joined)
	height = lipgloss.Height(content)

	m.setCachedRender(content, cappedWidth, height)
	return m.renderHighlighted(content, cappedWidth, height)
}

// Render implements MessageItem.
func (m *TerminalOutputItem) Render(width int) string {
	return m.RawRender(width)
}
