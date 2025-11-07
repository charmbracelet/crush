package quit

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
)

const (
	question                      = "Are you sure you want to quit?"
	QuitDialogID dialogs.DialogID = "quit"
)

// QuitDialog represents a confirmation dialog for quitting the application.
type QuitDialog interface {
	dialogs.DialogModel
}

type quitDialogCmp struct {
	wWidth  int
	wHeight int

	selectedNo bool // true if "No" button is selected
	keymap     KeyMap
}

// NewQuitDialog creates a new quit confirmation dialog.
func NewQuitDialog() QuitDialog {
	return &quitDialogCmp{
		selectedNo: true, // Default to "No" for safety
		keymap:     DefaultKeymap(),
	}
}

func (q *quitDialogCmp) Init() tea.Cmd {
	return nil
}

// Update handles keyboard input for the quit dialog.
func (q *quitDialogCmp) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		q.wWidth = msg.Width
		q.wHeight = msg.Height
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, q.keymap.LeftRight, q.keymap.Tab):
			q.selectedNo = !q.selectedNo
			return q, nil
		case key.Matches(msg, q.keymap.EnterSpace):
			if !q.selectedNo {
				return q, tea.Quit
			}
			return q, util.CmdHandler(dialogs.CloseDialogMsg{})
		case key.Matches(msg, q.keymap.Yes):
			return q, tea.Quit
		case key.Matches(msg, q.keymap.No, q.keymap.Close):
			return q, util.CmdHandler(dialogs.CloseDialogMsg{})
		}
	}
	return q, nil
}

// View renders the quit dialog with Yes/No buttons.
func (q *quitDialogCmp) View() string {
	t := styles.CurrentTheme()
	baseStyle := t.S().Base
	yesStyle := t.S().Text
	noStyle := yesStyle

	if q.selectedNo {
		noStyle = noStyle.Foreground(t.White).Background(t.Secondary)
		yesStyle = yesStyle.Background(t.BgSubtle)
	} else {
		yesStyle = yesStyle.Foreground(t.White).Background(t.Secondary)
		noStyle = noStyle.Background(t.BgSubtle)
	}

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

	quitDialogStyle := baseStyle.
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus)

	return quitDialogStyle.Render(content)
}

func (q *quitDialogCmp) Position() (int, int) {
	row := q.wHeight / 2
	row -= 7 / 2
	col := q.wWidth / 2
	col -= (lipgloss.Width(question) + 4) / 2

	return row, col
}

func (q *quitDialogCmp) ID() dialogs.DialogID {
	return QuitDialogID
}
