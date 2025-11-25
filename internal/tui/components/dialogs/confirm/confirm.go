package confirm

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
)

// ResultMsg is sent when user confirms or cancels
type ResultMsg struct {
	Confirmed bool
	Context   string // Identifies what's being confirmed (e.g., "delete-provider:openai")
}

type KeyMap struct {
	Left, Right, Confirm, Close, Tab, ShiftTab, Yes, No key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "select yes"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "select no"),
		),
		Confirm: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "confirm"),
		),
		Close: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "toggle"),
		),
		ShiftTab: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "toggle"),
		),
		Yes: key.NewBinding(
			key.WithKeys("y", "Y"),
			key.WithHelp("y/Y", "yes"),
		),
		No: key.NewBinding(
			key.WithKeys("n", "N"),
			key.WithHelp("n/N", "no"),
		),
	}
}

type Model struct {
	title           string
	message         string
	context         string
	confirmSelected bool // true=Yes, false=No
	width           int
	wWidth          int
	wHeight         int
	keyMap          KeyMap
}

func New(title, message, context string) *Model {
	return &Model{
		title:           title,
		message:         message,
		context:         context,
		confirmSelected: false, // Default to "No"
		width:           50,
		keyMap:          DefaultKeyMap(),
	}
}

func (c *Model) Init() tea.Cmd {
	return nil
}

func (c *Model) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		c.wWidth = msg.Width
		c.wHeight = msg.Height
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, c.keyMap.Left):
			c.confirmSelected = true // Yes
		case key.Matches(msg, c.keyMap.Right):
			c.confirmSelected = false // No
		case key.Matches(msg, c.keyMap.Tab, c.keyMap.ShiftTab):
			c.confirmSelected = !c.confirmSelected // Toggle
		case key.Matches(msg, c.keyMap.Yes):
			c.confirmSelected = true
			return c, util.CmdHandler(ResultMsg{
				Confirmed: true,
				Context:   c.context,
			})
		case key.Matches(msg, c.keyMap.No):
			c.confirmSelected = false
			return c, util.CmdHandler(ResultMsg{
				Confirmed: false,
				Context:   c.context,
			})
		case key.Matches(msg, c.keyMap.Confirm):
			return c, util.CmdHandler(ResultMsg{
				Confirmed: c.confirmSelected,
				Context:   c.context,
			})
		case key.Matches(msg, c.keyMap.Close):
			return c, util.CmdHandler(ResultMsg{
				Confirmed: false,
				Context:   c.context,
			})
		}
	}
	return c, nil
}

func (c *Model) View() string {
	t := styles.CurrentTheme()

	// Render buttons using core helper
	buttons := core.SelectableButtons([]core.ButtonOpts{
		{
			Text:           "Yes",
			UnderlineIndex: 0,
			Selected:       c.confirmSelected,
		},
		{
			Text:           "No",
			UnderlineIndex: 0,
			Selected:       !c.confirmSelected,
		},
	}, "  ")

	// Render content
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		t.S().Base.Bold(true).Padding(0, 1).Render(c.title),
		"",
		t.S().Base.Padding(0, 1).Render(c.message),
		"",
		t.S().Base.Width(c.width-2).AlignHorizontal(lipgloss.Center).Render(buttons),
		"",
		t.S().Subtle.Padding(0, 1).Render("tab/arrows to select • enter/y/n to confirm • esc cancel"),
	)

	dialog := t.S().Base.
		Width(c.width).
		Padding(1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Render(content)

	return dialog
}

func (c *Model) Position() (int, int) {
	row := c.wHeight / 2
	row -= 5 // Approximate half height of dialog
	col := c.wWidth / 2
	col -= c.width / 2
	return row, col
}

func (c *Model) ID() dialogs.DialogID {
	return dialogs.DialogID("confirm-" + c.context)
}
