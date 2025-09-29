package quit

import (
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/lipgloss/v2"
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
func (q *quitDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
	// Render complete button text with brackets in one go
	yesButton := yesStyle.PaddingLeft(horizontalPadding).Render("[Y]es, quit Crush") +
		yesStyle.PaddingRight(horizontalPadding).Render("")
	noButton := noStyle.PaddingLeft(horizontalPadding).Render("[N]o, continue") +
		noStyle.PaddingRight(horizontalPadding).Render("")

	// Calculate the total width needed for centered layout
	yesButtonWidth := lipgloss.Width(yesButton)
	noButtonWidth := lipgloss.Width(noButton)
	totalButtonsWidth := yesButtonWidth + noButtonWidth + 6 // 6 for spacing

	// Create a centered container for the buttons
	buttons := baseStyle.Width(totalButtonsWidth).Align(lipgloss.Center).Render(
		lipgloss.JoinHorizontal(lipgloss.Center, yesButton, "   ", noButton),
	)

	// Calculate the maximum width for proper centering
	maxWidth := max(lipgloss.Width(question), totalButtonsWidth)

	content := baseStyle.Width(maxWidth).Align(lipgloss.Center).Render(
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

	// Calculate dialog width more accurately
	const horizontalPadding = 3
	yesButtonWidth := lipgloss.Width("[Y]es, quit Crush") + horizontalPadding
	noButtonWidth := lipgloss.Width("[N]o, continue") + horizontalPadding
	totalButtonsWidth := yesButtonWidth + noButtonWidth + 6
	dialogWidth := max(lipgloss.Width(question), totalButtonsWidth) + 4 // +4 for padding

	col -= dialogWidth / 2

	return row, col
}

func (q *quitDialogCmp) ID() dialogs.DialogID {
	return QuitDialogID
}
