package sessions

import (
	"fmt"

	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/lipgloss/v2"
)

const (
	DeleteConfirmationDialogID dialogs.DialogID = "delete_confirmation"
	question                                    = "Delete session? [y/N]"
)

// DeleteConfirmationDialog represents a confirmation dialog for deleting a session
type DeleteConfirmationDialog interface {
	dialogs.DialogModel
}

type deleteConfirmationDialogCmp struct {
	wWidth       int
	wHeight      int
	sessionID    string
	sessionTitle string
	selectedNo   bool // true if "No" button is selected
	keymap       KeyMap
}

// NewDeleteConfirmationDialog creates a new delete confirmation dialog
func NewDeleteConfirmationDialog(sessionID, sessionTitle string) DeleteConfirmationDialog {
	return &deleteConfirmationDialogCmp{
		sessionID:    sessionID,
		sessionTitle: sessionTitle,
		selectedNo:   true, // Default to "No" for safety
		keymap:       DefaultDeleteConfirmationKeyMap(),
	}
}

func (d *deleteConfirmationDialogCmp) Init() tea.Cmd {
	return nil
}

// Update handles keyboard input for the delete confirmation dialog
func (d *deleteConfirmationDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		d.wWidth = msg.Width
		d.wHeight = msg.Height
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, d.keymap.LeftRight, d.keymap.Tab):
			d.selectedNo = !d.selectedNo
			return d, nil
		case key.Matches(msg, d.keymap.EnterSpace):
			if !d.selectedNo {
				// User confirmed - send delete session message
				return d, util.CmdHandler(DeleteSessionConfirmedMsg{SessionID: d.sessionID})
			}
			return d, util.CmdHandler(dialogs.CloseDialogMsg{})
		case key.Matches(msg, d.keymap.Yes):
			// User confirmed - send delete session message
			return d, util.CmdHandler(DeleteSessionConfirmedMsg{SessionID: d.sessionID})
		case key.Matches(msg, d.keymap.No, d.keymap.Close):
			return d, util.CmdHandler(dialogs.CloseDialogMsg{})
		}
	}
	return d, nil
}

// View renders the delete confirmation dialog with Yes/No buttons
func (d *deleteConfirmationDialogCmp) View() string {
	t := styles.CurrentTheme()
	baseStyle := t.S().Base
	yesStyle := t.S().Text
	noStyle := yesStyle

	if d.selectedNo {
		noStyle = noStyle.Foreground(t.White).Background(t.Secondary)
		yesStyle = yesStyle.Background(t.BgSubtle)
	} else {
		yesStyle = yesStyle.Foreground(t.White).Background(t.Secondary)
		noStyle = noStyle.Background(t.BgSubtle)
	}

	const horizontalPadding = 3
	yesButton := yesStyle.PaddingLeft(horizontalPadding).Underline(true).Render("Y") +
		yesStyle.PaddingRight(horizontalPadding).Render("es")
	noButton := noStyle.PaddingLeft(horizontalPadding).Underline(true).Render("N") +
		noStyle.PaddingRight(horizontalPadding).Render("o")

	buttons := baseStyle.Width(lipgloss.Width(question)).Align(lipgloss.Right).Render(
		lipgloss.JoinHorizontal(lipgloss.Center, yesButton, "  ", noButton),
	)

	content := baseStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Center,
			fmt.Sprintf("Delete session %q?", d.sessionTitle),
			"",
			buttons,
		),
	)

	dialogStyle := baseStyle.
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus)

	return dialogStyle.Render(content)
}

func (d *deleteConfirmationDialogCmp) Position() (int, int) {
	row := d.wHeight / 2
	row -= 7 / 2
	col := d.wWidth / 2
	col -= (lipgloss.Width(question) + 4) / 2

	return row, col
}

func (d *deleteConfirmationDialogCmp) ID() dialogs.DialogID {
	return DeleteConfirmationDialogID
}
