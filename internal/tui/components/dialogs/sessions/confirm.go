package sessions

import (
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/lipgloss/v2"
)

const ConfirmDeleteDialogID dialogs.DialogID = "confirm_delete"

// DeleteConfirmMsg is sent when the user confirms deletion
type DeleteConfirmMsg struct {
	SessionID string
}

// DeleteCancelMsg is sent when the user cancels deletion
type DeleteCancelMsg struct{}

// ConfirmDeleteDialog represents a confirmation dialog for deleting a session.
type ConfirmDeleteDialog interface {
	dialogs.DialogModel
}

type confirmDeleteDialogCmp struct {
	wWidth         int
	wHeight        int
	session        session.Session
	selectedNo     bool // true if "No" button is selected
	keymap         KeyMap
	parentDialogID dialogs.DialogID
}

// NewConfirmDeleteDialog creates a new delete confirmation dialog.
func NewConfirmDeleteDialog(session session.Session, parentDialogID dialogs.DialogID) ConfirmDeleteDialog {
	return &confirmDeleteDialogCmp{
		session:        session,
		selectedNo:     true, // Default to "No" for safety
		keymap:         DefaultKeyMap(),
		parentDialogID: parentDialogID,
	}
}

func (c *confirmDeleteDialogCmp) Init() tea.Cmd {
	return nil
}

// Update handles keyboard input for the delete confirmation dialog.
func (c *confirmDeleteDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		c.wWidth = msg.Width
		c.wHeight = msg.Height
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, c.keymap.LeftRight):
			c.selectedNo = !c.selectedNo
			return c, nil
		case key.Matches(msg, c.keymap.EnterSpace):
			if !c.selectedNo {
				return c, tea.Sequence(
					util.CmdHandler(dialogs.CloseDialogMsg{}),
					util.CmdHandler(DeleteConfirmMsg{SessionID: c.session.ID}),
				)
			}
			return c, util.CmdHandler(dialogs.CloseDialogMsg{})
		case key.Matches(msg, c.keymap.Yes):
			return c, tea.Sequence(
				util.CmdHandler(dialogs.CloseDialogMsg{}),
				util.CmdHandler(DeleteConfirmMsg{SessionID: c.session.ID}),
			)
		case key.Matches(msg, c.keymap.No, c.keymap.Close):
			return c, util.CmdHandler(dialogs.CloseDialogMsg{})
		}
	}
	return c, nil
}

// View renders the delete confirmation dialog with Yes/No buttons.
func (c *confirmDeleteDialogCmp) View() string {
	t := styles.CurrentTheme()
	baseStyle := t.S().Base
	yesStyle := t.S().Text
	noStyle := yesStyle

	if c.selectedNo {
		noStyle = noStyle.Foreground(t.White).Background(t.Secondary)
		yesStyle = yesStyle.Background(t.BgSubtle)
	} else {
		yesStyle = yesStyle.Foreground(t.White).Background(t.Secondary)
		noStyle = noStyle.Background(t.BgSubtle)
	}

	question := "Delete session \"" + c.session.Title + "\"?"
	const horizontalPadding = 3
	yesButton := yesStyle.PaddingLeft(horizontalPadding).Underline(true).Render("Y") +
		yesStyle.PaddingRight(horizontalPadding).Render("ep!")
	noButton := noStyle.PaddingLeft(horizontalPadding).Underline(true).Render("N") +
		noStyle.PaddingRight(horizontalPadding).Render("ope")

	buttons := baseStyle.Width(lipgloss.Width(question)).Align(lipgloss.Right).Render(
		lipgloss.JoinHorizontal(lipgloss.Center, yesButton, "  ", noButton),
	)

	content := baseStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Center,
			question,
			"",
			buttons,
		),
	)

	confirmDialogStyle := baseStyle.
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus)

	return confirmDialogStyle.Render(content)
}

func (c *confirmDeleteDialogCmp) Position() (int, int) {
	question := "Delete session \"" + c.session.Title + "\"?"
	row := c.wHeight / 2
	row -= 7 / 2
	col := c.wWidth / 2
	col -= (lipgloss.Width(question) + 4) / 2

	return row, col
}

func (c *confirmDeleteDialogCmp) ID() dialogs.DialogID {
	return ConfirmDeleteDialogID
}
