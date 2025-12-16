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
		modeButtons,
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

	// Calculate button width to match list items exactly
	// listWidth = width - 2 (account for border), same as session dialog
	// This is the width passed to SetSize() for list items
	buttonWidth := m.width - 2

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

	var parts []string
	for _, button := range buttons {
		// Use exact same styling as list items (completionItemCmp)
		// itemStyle with Width and Padding ensures full-width background
		itemStyle := t.S().Base.Padding(0, 1).Width(buttonWidth)
		titleStyle := t.S().Text
		titleMatchStyle := t.S().Text.Underline(true)

		if button.Selected {
			titleStyle = t.S().TextSelected
			titleMatchStyle = t.S().TextSelected.Underline(true)
			itemStyle = itemStyle.Background(t.Primary)
		}

		// Create the button text with underlined character
		text := button.Text
		var message string
		if button.UnderlineIndex >= 0 && button.UnderlineIndex < len(text) {
			before := text[:button.UnderlineIndex]
			underlined := text[button.UnderlineIndex : button.UnderlineIndex+1]
			after := text[button.UnderlineIndex+1:]
			message = titleStyle.Render(before) +
				titleMatchStyle.Render(underlined) +
				titleStyle.Render(after)
		} else {
			message = titleStyle.Render(text)
		}

		// Render exactly like list items: itemStyle.Render() with JoinHorizontal
		// This ensures the background fills the full width
		buttonContent := itemStyle.Render(
			lipgloss.JoinHorizontal(
				lipgloss.Left,
				message,
			),
		)
		parts = append(parts, buttonContent)
	}

	// Join vertically with left alignment
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}
