package messages

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/ordered"
	"github.com/google/uuid"

	"github.com/charmbracelet/crush/internal/tui/components/core/layout"
	"github.com/charmbracelet/crush/internal/tui/exp/list"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
)

// TerminalOutputCmp defines the interface for terminal output display components.
type TerminalOutputCmp interface {
	util.Model
	layout.Sizeable
	layout.Focusable
	list.Item
}

// terminalOutputCmp displays the result of a terminal command execution.
type terminalOutputCmp struct {
	id       string
	width    int
	focused  bool
	command  string
	stdout   string
	stderr   string
	exitCode int
	duration time.Duration
	hasError bool
}

// TerminalOutputData contains the data needed to create a terminal output component.
type TerminalOutputData struct {
	Command  string
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
	Error    error
}

// NewTerminalOutputCmp creates a new terminal output component.
func NewTerminalOutputCmp(data TerminalOutputData) TerminalOutputCmp {
	return &terminalOutputCmp{
		id:       uuid.NewString(),
		command:  data.Command,
		stdout:   data.Stdout,
		stderr:   data.Stderr,
		exitCode: data.ExitCode,
		duration: data.Duration,
		hasError: data.Error != nil || data.ExitCode != 0,
	}
}

func (m *terminalOutputCmp) ID() string {
	return m.id
}

func (m *terminalOutputCmp) Init() tea.Cmd {
	return nil
}

func (m *terminalOutputCmp) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	return m, nil
}

func (m *terminalOutputCmp) View() string {
	t := styles.CurrentTheme()
	textWidth := m.width - 4 // Account for border and padding

	// Header with command
	cmdIcon := t.S().Base.
		Foreground(t.BgSubtle).
		Background(t.Yellow).
		Padding(0, 1).
		Bold(true).
		Render("$")

	cmdText := t.S().Base.
		Foreground(t.FgBase).
		Bold(true).
		Render(" " + m.command)

	header := lipgloss.JoinHorizontal(lipgloss.Left, cmdIcon, cmdText)

	// Output content
	var outputParts []string
	outputParts = append(outputParts, header)
	outputParts = append(outputParts, "")

	// Combine stdout and stderr
	output := strings.TrimSpace(m.stdout)
	if m.stderr != "" {
		if output != "" {
			output += "\n"
		}
		output += strings.TrimSpace(m.stderr)
	}

	if output == "" {
		output = t.S().Subtle.Render("(no output)")
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

	outputStyle := t.S().Base.
		Foreground(t.FgMuted).
		Width(textWidth)

	outputParts = append(outputParts, outputStyle.Render(output))

	// Footer with execution info
	var footerParts []string

	// Duration
	durationText := t.S().Subtle.Render(m.duration.Round(time.Millisecond).String())
	footerParts = append(footerParts, durationText)

	// Exit code
	if m.exitCode != 0 {
		exitStyle := t.S().Base.
			Foreground(t.Red).
			Bold(true)
		footerParts = append(footerParts, exitStyle.Render(fmt.Sprintf("exit %d", m.exitCode)))
	}

	if len(footerParts) > 0 {
		outputParts = append(outputParts, "")
		footer := t.S().Base.PaddingLeft(1).Render(strings.Join(footerParts, " • "))
		outputParts = append(outputParts, footer)
	}

	joined := lipgloss.JoinVertical(lipgloss.Left, outputParts...)

	// Container style - use a distinct terminal-like appearance
	borderColor := t.Green
	if m.hasError {
		borderColor = t.Red
	}
	if m.focused {
		borderColor = t.Primary
	}

	containerStyle := t.S().Base.
		PaddingLeft(1).
		BorderLeft(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(borderColor)

	return containerStyle.Render(joined)
}

// Blur removes focus from the component.
func (m *terminalOutputCmp) Blur() tea.Cmd {
	m.focused = false
	return nil
}

// Focus sets focus on the component.
func (m *terminalOutputCmp) Focus() tea.Cmd {
	m.focused = true
	return nil
}

// IsFocused returns whether the component is currently focused.
func (m *terminalOutputCmp) IsFocused() bool {
	return m.focused
}

// GetSize returns the current dimensions.
func (m *terminalOutputCmp) GetSize() (int, int) {
	return m.width, 0
}

// SetSize updates the width of the component.
func (m *terminalOutputCmp) SetSize(width int, height int) tea.Cmd {
	m.width = ordered.Clamp(width, 1, 120)
	return nil
}
