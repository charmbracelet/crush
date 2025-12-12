package queuemode

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
)

const (
	QueueModeDialogID dialogs.DialogID = "queue_mode"

	defaultWidth = 50
)

// QueueModeSelectedMsg is sent when a queue mode is selected
type QueueModeSelectedMsg struct {
	DeferredQueue bool
}

// QueueModeDialog interface for the queue mode selection dialog
type QueueModeDialog interface {
	dialogs.DialogModel
}

type queueModeDialogCmp struct {
	width   int
	wWidth  int
	wHeight int

	selectedMode bool // true = queue mode, false = interrupt mode
	keyMap       KeyMap
	help         help.Model
}

func NewQueueModeDialogCmp() QueueModeDialog {
	keyMap := DefaultKeyMap()
	help := help.New()
	t := styles.CurrentTheme()
	help.Styles = t.S().Help

	cfg := config.Get()
	deferredQueue := false
	if cfg.Options != nil {
		deferredQueue = cfg.Options.DeferredQueue
	}

	return &queueModeDialogCmp{
		width:        defaultWidth,
		keyMap:       keyMap,
		help:         help,
		selectedMode: deferredQueue,
	}
}

func (m *queueModeDialogCmp) Init() tea.Cmd {
	return nil
}

func (m *queueModeDialogCmp) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.wWidth = msg.Width
		m.wHeight = msg.Height
		m.help.SetWidth(m.width - 2)
		return m, nil
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.Select):
			return m, tea.Sequence(
				util.CmdHandler(dialogs.CloseDialogMsg{}),
				util.CmdHandler(QueueModeSelectedMsg{
					DeferredQueue: m.selectedMode,
				}),
			)
		case key.Matches(msg, m.keyMap.Toggle):
			m.selectedMode = !m.selectedMode
			return m, nil
		case key.Matches(msg, m.keyMap.Close):
			return m, util.CmdHandler(dialogs.CloseDialogMsg{})
		}
	}
	return m, nil
}

func (m *queueModeDialogCmp) View() string {
	t := styles.CurrentTheme()

	modeButtons := m.modeButtons()

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		t.S().Base.Padding(0, 1, 1, 1).Render(core.Title("Queue Mode", m.width-4)),
		"",
		t.S().Base.PaddingLeft(1).Render(modeButtons),
		"",
		t.S().Base.Width(m.width-2).PaddingLeft(1).AlignHorizontal(lipgloss.Left).Render(m.help.View(m.keyMap)),
	)
	return m.style().Render(content)
}

func (m *queueModeDialogCmp) Cursor() *tea.Cursor {
	return nil
}

func (m *queueModeDialogCmp) style() lipgloss.Style {
	t := styles.CurrentTheme()
	return t.S().Base.
		Width(m.width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus)
}

func (m *queueModeDialogCmp) Position() (int, int) {
	row := m.wHeight/4 - 2
	col := m.wWidth / 2
	col -= m.width / 2
	return row, col
}

func (m *queueModeDialogCmp) ID() dialogs.DialogID {
	return QueueModeDialogID
}

func (m *queueModeDialogCmp) modeButtons() string {
	t := styles.CurrentTheme()

	buttons := []struct {
		Text           string
		UnderlineIndex int
		Selected       bool
	}{
		{
			Text:           "Interrupt Mode",
			UnderlineIndex: 0, // "I"
			Selected:       !m.selectedMode,
		},
		{
			Text:           "Queue Mode",
			UnderlineIndex: 0, // "Q"
			Selected:       m.selectedMode,
		},
	}

	// Calculate the maximum text width to determine button size
	maxTextWidth := 0
	for _, btn := range buttons {
		textWidth := lipgloss.Width(btn.Text)
		if textWidth > maxTextWidth {
			maxTextWidth = textWidth
		}
	}
	// Button width = text width + padding (2 on each side = 4 total) + left offset for selected (2 spaces)
	buttonWidth := maxTextWidth + 4 + 2

	// Render each button with fixed width, adding left offset for selected button to align text
	var parts []string
	for _, button := range buttons {
		// Use Primary background for selected button (like menu items), BgSubtle for unselected
		itemStyle := t.S().Base.Padding(0, 1).Width(buttonWidth)
		textStyle := t.S().Text

		if button.Selected {
			itemStyle = itemStyle.Background(t.Primary)
			textStyle = t.S().TextSelected
		} else {
			itemStyle = itemStyle.Background(t.BgSubtle)
		}

		// Create the button text with underlined character
		text := button.Text
		var message string
		if button.UnderlineIndex >= 0 && button.UnderlineIndex < len(text) {
			before := text[:button.UnderlineIndex]
			underlined := text[button.UnderlineIndex : button.UnderlineIndex+1]
			after := text[button.UnderlineIndex+1:]
			message = textStyle.Render(before) +
				textStyle.Underline(true).Render(underlined) +
				textStyle.Render(after)
		} else {
			message = textStyle.Render(text)
		}

		// Add left offset (2 spaces) for selected button to align text with unselected button
		// This ensures text alignment between selected and unselected buttons
		leftOffset := ""
		if button.Selected {
			leftOffset = "  " // 2 spaces offset for selected button
		}

		// Render the button with padding and left offset
		buttonContent := itemStyle.Render(leftOffset + message)

		parts = append(parts, buttonContent)
	}

	// Join vertically
	buttonsContent := lipgloss.JoinVertical(lipgloss.Left, parts...)
	return t.S().Base.Width(buttonWidth).Render(buttonsContent)
}
