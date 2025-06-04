package quit

import (
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/opencode-ai/opencode/internal/tui/components/dialogs"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

const (
	question                  = "Are you sure you want to quit?"
	id       dialogs.DialogID = "quit"
)

// QuitDialog represents a confirmation dialog for quitting the application.
type QuitDialog interface {
	dialogs.DialogModel
	layout.Bindings
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
		case key.Matches(msg, q.keymap.LeftRight) || key.Matches(msg, q.keymap.Tab):
			q.selectedNo = !q.selectedNo
			return q, nil
		case key.Matches(msg, q.keymap.EnterSpace):
			if !q.selectedNo {
				return q, tea.Quit
			}
			return q, util.CmdHandler(dialogs.CloseDialogMsg{})
		case key.Matches(msg, q.keymap.Yes):
			return q, tea.Quit
		case key.Matches(msg, q.keymap.No):
			return q, util.CmdHandler(dialogs.CloseDialogMsg{})
		}
	}
	return q, nil
}

// View renders the quit dialog with Yes/No buttons.
func (q *quitDialogCmp) View() tea.View {
	t := styles.CurrentTheme()
	baseStyle := t.S().Base
	yesStyle := t.S().Text
	noStyle := yesStyle

	if q.selectedNo {
		noStyle = noStyle.Background(t.Primary)
		yesStyle = yesStyle.Background(t.BgSubtle)
	} else {
		yesStyle = yesStyle.Background(t.Primary)
		noStyle = noStyle.Background(t.BgSubtle)
	}

	yesButton := yesStyle.Padding(0, 1).Render("Yes")
	noButton := noStyle.Padding(0, 1).Render("No")

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

	return tea.NewView(
		quitDialogStyle.Render(content),
	)
}

func (q *quitDialogCmp) BindingKeys() []key.Binding {
	return layout.KeyMapToSlice(q.keymap)
}

func (q *quitDialogCmp) Position() (int, int) {
	row := q.wHeight / 2
	row -= 7 / 2
	col := q.wWidth / 2
	col -= (lipgloss.Width(question) + 4) / 2

	return row, col
}

func (q *quitDialogCmp) ID() dialogs.DialogID {
	return id
}
