package commands

import (
	"github.com/charmbracelet/bubbles/v2/textinput"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"github.com/opencode-ai/opencode/internal/logging"
	"github.com/opencode-ai/opencode/internal/tui/components/core/list"
	"github.com/opencode-ai/opencode/internal/tui/components/dialogs"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

const (
	id dialogs.DialogID = "commands"
)

// Command represents a command that can be executed
type Command struct {
	ID          string
	Title       string
	Description string
	Handler     func(cmd Command) tea.Cmd
}

// CommandsDialog represents the commands dialog.
type CommandsDialog interface {
	dialogs.DialogModel
}

type commandDialogCmp struct {
	width   int
	wWidth  int // Width of the terminal window
	wHeight int // Height of the terminal window

	commandList list.ListModel
	input       textinput.Model
	oldCursor   tea.Cursor
}

func NewCommandDialog() CommandsDialog {
	ti := textinput.New()
	ti.Placeholder = "Type a command or search..."
	ti.SetVirtualCursor(false)
	ti.Focus()
	ti.SetWidth(60 - 7)
	commandList := list.New()
	return &commandDialogCmp{
		commandList: commandList,
		width:       60,
		input:       ti,
	}
}

func (c *commandDialogCmp) Init() tea.Cmd {
	logging.Info("Initializing commands dialog")
	commands, err := LoadCustomCommands()
	if err != nil {
		return util.ReportError(err)
	}
	logging.Info("Commands loaded", "count", len(commands))

	commandItems := make([]util.Model, 0, len(commands))

	for _, cmd := range commands {
		commandItems = append(commandItems, NewCommandItem(cmd))
	}
	c.commandList.SetItems(commandItems)
	return c.commandList.Init()
}

func (c *commandDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		c.wWidth = msg.Width
		c.wHeight = msg.Height
		return c, c.commandList.SetSize(60, min(len(c.commandList.Items())*2, c.wHeight/2))
	}
	u, cmd := c.input.Update(msg)
	c.input = u
	return c, cmd
}

func (c *commandDialogCmp) View() tea.View {
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		c.inputStyle().Render(c.input.View()),
		c.commandList.View().String(),
	)

	v := tea.NewView(c.style().Render(content))
	v.SetCursor(c.getCursor())
	return v
}

func (c *commandDialogCmp) getCursor() *tea.Cursor {
	cursor := c.input.Cursor()
	offset := 10 + 1
	cursor.Y += offset
	_, col := c.Position()
	cursor.X = c.input.Cursor().X + col + 2
	return cursor
}

func (c *commandDialogCmp) inputStyle() lipgloss.Style {
	t := theme.CurrentTheme()
	return styles.BaseStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(t.TextMuted()).
		BorderBackground(t.Background()).
		BorderBottom(true)
}

func (c *commandDialogCmp) style() lipgloss.Style {
	t := theme.CurrentTheme()
	return styles.BaseStyle().
		Width(c.width).
		Padding(0, 1, 1, 1).
		Border(lipgloss.RoundedBorder()).
		BorderBackground(t.Background()).
		BorderForeground(t.TextMuted())
}

func (q *commandDialogCmp) Position() (int, int) {
	row := 10
	col := q.wWidth / 2
	col -= q.width / 2
	return row, col
}

func (c *commandDialogCmp) ID() dialogs.DialogID {
	return id
}
