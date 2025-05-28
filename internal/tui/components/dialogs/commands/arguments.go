package commands

import (
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/opencode-ai/opencode/internal/tui/components/dialogs"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
)

const (
	argumentsDialogID dialogs.DialogID = "arguments"
)

// ShowArgumentsDialogMsg is a message that is sent to show the arguments dialog.
type ShowArgumentsDialogMsg struct {
	CommandID string
	Content   string
	ArgNames  []string
}

// CloseArgumentsDialogMsg is a message that is sent when the arguments dialog is closed.
type CloseArgumentsDialogMsg struct {
	Submit    bool
	CommandID string
	Content   string
	Args      map[string]string
}

// CommandArgumentsDialog represents the commands dialog.
type CommandArgumentsDialog interface {
	dialogs.DialogModel
}

type commandArgumentsDialogCmp struct {
	width   int
	wWidth  int // Width of the terminal window
	wHeight int // Height of the terminal window
}

func NewCommandArgumentsDialog() CommandArgumentsDialog {
	return &commandArgumentsDialogCmp{}
}

// Init implements CommandArgumentsDialog.
func (c *commandArgumentsDialogCmp) Init() tea.Cmd {
	return nil
}

// Update implements CommandArgumentsDialog.
func (c *commandArgumentsDialogCmp) Update(tea.Msg) (tea.Model, tea.Cmd) {
	return c, nil
}

// View implements CommandArgumentsDialog.
func (c *commandArgumentsDialogCmp) View() tea.View {
	return tea.NewView("")
}

func (c *commandArgumentsDialogCmp) moveCursor(cursor *tea.Cursor) *tea.Cursor {
	offset := 10 + 1
	cursor.Y += offset
	_, col := c.Position()
	cursor.X = cursor.X + col + 2
	return cursor
}

func (c *commandArgumentsDialogCmp) style() lipgloss.Style {
	t := theme.CurrentTheme()
	return styles.BaseStyle().
		Width(c.width).
		Padding(1).
		Border(lipgloss.RoundedBorder()).
		BorderBackground(t.Background()).
		BorderForeground(t.TextMuted())
}

func (q *commandArgumentsDialogCmp) Position() (int, int) {
	row := 10
	col := q.wWidth / 2
	col -= q.width / 2
	return row, col
}

// ID implements CommandArgumentsDialog.
func (c *commandArgumentsDialogCmp) ID() dialogs.DialogID {
	return argumentsDialogID
}
