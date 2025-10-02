package compact

import (
	"context"

	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
)

const CompactDialogID dialogs.DialogID = "compact"

// CompactDialog interface for the session compact dialog
type CompactDialog interface {
	dialogs.DialogModel
}

type compactDialogCmp struct {
	wWidth, wHeight int
	width, height   int
	selected        int
	keyMap          KeyMap
	sessionID       string
	progress        string
	agent           agent.Coordinator
	noAsk           bool // If true, skip confirmation dialog
}

// NewCompactDialogCmp creates a new session compact dialog
func NewCompactDialogCmp(agent agent.Coordinator, sessionID string, noAsk bool) CompactDialog {
	return &compactDialogCmp{
		sessionID: sessionID,
		keyMap:    DefaultKeyMap(),
		selected:  0,
		agent:     agent,
		noAsk:     noAsk,
	}
}

func (c *compactDialogCmp) Init() tea.Cmd {
	if c.noAsk {
		// If noAsk is true, skip confirmation and start compaction immediately
		c.agent.Summarize(context.Background(), c.sessionID)
		return util.CmdHandler(dialogs.CloseDialogMsg{})
	}
	return nil
}

func (c *compactDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		c.wWidth = msg.Width
		c.wHeight = msg.Height
		cmd := c.SetSize()
		return c, cmd
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, c.keyMap.ChangeSelection):
			c.selected = (c.selected + 1) % 2
			return c, nil
		case key.Matches(msg, c.keyMap.Select):
			if c.selected == 0 {
				c.agent.Summarize(context.Background(), c.sessionID)
				return c, util.CmdHandler(dialogs.CloseDialogMsg{})
			} else {
				return c, util.CmdHandler(dialogs.CloseDialogMsg{})
			}
		case key.Matches(msg, c.keyMap.Y):
			c.agent.Summarize(context.Background(), c.sessionID)
			return c, util.CmdHandler(dialogs.CloseDialogMsg{})
		case key.Matches(msg, c.keyMap.N):
			return c, util.CmdHandler(dialogs.CloseDialogMsg{})
		case key.Matches(msg, c.keyMap.Close):
			return c, util.CmdHandler(dialogs.CloseDialogMsg{})
		}
	}
	return c, nil
}

func (c *compactDialogCmp) renderButtons() string {
	t := styles.CurrentTheme()
	baseStyle := t.S().Base

	buttons := []core.ButtonOpts{
		{
			Text:           "Yes",
			UnderlineIndex: 0, // "Y"
			Selected:       c.selected == 0,
		},
		{
			Text:           "No",
			UnderlineIndex: 0, // "N"
			Selected:       c.selected == 1,
		},
	}

	content := core.SelectableButtons(buttons, "  ")

	return baseStyle.AlignHorizontal(lipgloss.Right).Width(c.width - 4).Render(content)
}

func (c *compactDialogCmp) render() string {
	t := styles.CurrentTheme()
	baseStyle := t.S().Base

	title := "Compact Session"
	titleView := core.Title(title, c.width-4)
	explanation := t.S().Text.
		Width(c.width - 4).
		Render("This will summarize the current session and reset the context. The conversation history will be condensed into a summary to free up context space while preserving important information.")

	question := t.S().Text.
		Width(c.width - 4).
		Render("Do you want to continue?")

	content := baseStyle.Render(lipgloss.JoinVertical(
		lipgloss.Left,
		explanation,
		"",
		question,
	))

	buttons := c.renderButtons()
	dialogContent := lipgloss.JoinVertical(
		lipgloss.Top,
		titleView,
		"",
		content,
		"",
		buttons,
		"",
	)

	return baseStyle.
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Width(c.width).
		Render(dialogContent)
}

func (c *compactDialogCmp) View() string {
	return c.render()
}

// SetSize sets the size of the component.
func (c *compactDialogCmp) SetSize() tea.Cmd {
	c.width = min(90, c.wWidth)
	c.height = min(15, c.wHeight)
	return nil
}

func (c *compactDialogCmp) Position() (int, int) {
	row := (c.wHeight / 2) - (c.height / 2)
	col := (c.wWidth / 2) - (c.width / 2)
	return row, col
}

// ID implements CompactDialog.
func (c *compactDialogCmp) ID() dialogs.DialogID {
	return CompactDialogID
}
