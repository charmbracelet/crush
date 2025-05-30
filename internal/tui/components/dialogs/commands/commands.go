package commands

import (
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"github.com/opencode-ai/opencode/internal/tui/components/chat"
	"github.com/opencode-ai/opencode/internal/tui/components/completions"
	"github.com/opencode-ai/opencode/internal/tui/components/core/list"
	"github.com/opencode-ai/opencode/internal/tui/components/dialogs"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

const (
	commandsDialogID dialogs.DialogID = "commands"

	defaultWidth int = 60
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
	commands    []Command
	keyMap      CommandsDialogKeyMap
}

func NewCommandDialog() CommandsDialog {
	listKeyMap := list.DefaultKeyMap()
	keyMap := DefaultCommandsDialogKeyMap()

	listKeyMap.Down.SetEnabled(false)
	listKeyMap.Up.SetEnabled(false)
	listKeyMap.NDown.SetEnabled(false)
	listKeyMap.NUp.SetEnabled(false)
	listKeyMap.HalfPageDown.SetEnabled(false)
	listKeyMap.HalfPageUp.SetEnabled(false)
	listKeyMap.Home.SetEnabled(false)
	listKeyMap.End.SetEnabled(false)

	listKeyMap.DownOneItem = keyMap.Next
	listKeyMap.UpOneItem = keyMap.Previous

	commandList := list.New(list.WithFilterable(true), list.WithKeyMap(listKeyMap))
	return &commandDialogCmp{
		commandList: commandList,
		width:       defaultWidth,
		keyMap:      DefaultCommandsDialogKeyMap(),
	}
}

func (c *commandDialogCmp) Init() tea.Cmd {
	commands, err := LoadCustomCommands()
	if err != nil {
		return util.ReportError(err)
	}
	c.commands = commands

	commandItems := []util.Model{}
	if len(commands) > 0 {
		commandItems = append(commandItems, NewItemSection("Custom Commands"))
		for _, cmd := range commands {
			commandItems = append(commandItems, completions.NewCompletionItem(cmd.Title, cmd))
		}
	}

	commandItems = append(commandItems, NewItemSection("Default"))

	for _, cmd := range c.defaultCommands() {
		c.commands = append(c.commands, cmd)
		commandItems = append(commandItems, completions.NewCompletionItem(cmd.Title, cmd))
	}

	c.commandList.SetItems(commandItems)
	return c.commandList.Init()
}

func (c *commandDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		c.wWidth = msg.Width
		c.wHeight = msg.Height
		return c, c.commandList.SetSize(c.listWidth(), c.listHeight())
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, c.keyMap.Select):
			selectedItemInx := c.commandList.SelectedIndex()
			if selectedItemInx == list.NoSelection {
				return c, nil // No item selected, do nothing
			}
			items := c.commandList.Items()
			selectedItem := items[selectedItemInx].(completions.CompletionItem).Value().(Command)
			return c, tea.Sequence(
				util.CmdHandler(dialogs.CloseDialogMsg{}),
				selectedItem.Handler(selectedItem),
			)
		default:
			u, cmd := c.commandList.Update(msg)
			c.commandList = u.(list.ListModel)
			return c, cmd
		}
	}
	return c, nil
}

func (c *commandDialogCmp) View() tea.View {
	listView := c.commandList.View()
	v := tea.NewView(c.style().Render(listView.String()))
	if listView.Cursor() != nil {
		c := c.moveCursor(listView.Cursor())
		v.SetCursor(c)
	}
	return v
}

func (c *commandDialogCmp) listWidth() int {
	return defaultWidth - 4 // 4 for padding
}

func (c *commandDialogCmp) listHeight() int {
	listHeigh := len(c.commandList.Items()) + 2 + 4 // height based on items + 2 for the input + 4 for the sections
	return min(listHeigh, c.wHeight/2)
}

func (c *commandDialogCmp) moveCursor(cursor *tea.Cursor) *tea.Cursor {
	offset := 10 + 1
	cursor.Y += offset
	_, col := c.Position()
	cursor.X = cursor.X + col + 2
	return cursor
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

func (c *commandDialogCmp) defaultCommands() []Command {
	return []Command{
		{
			ID:          "init",
			Title:       "Initialize Project",
			Description: "Create/Update the OpenCode.md memory file",
			Handler: func(cmd Command) tea.Cmd {
				prompt := `Please analyze this codebase and create a OpenCode.md file containing:
	1. Build/lint/test commands - especially for running a single test
	2. Code style guidelines including imports, formatting, types, naming conventions, error handling, etc.

	The file you create will be given to agentic coding agents (such as yourself) that operate in this repository. Make it about 20 lines long.
	If there's already a opencode.md, improve it.
	If there are Cursor rules (in .cursor/rules/ or .cursorrules) or Copilot rules (in .github/copilot-instructions.md), make sure to include them.`
				return tea.Batch(
					util.CmdHandler(chat.SendMsg{
						Text: prompt,
					}),
				)
			},
		},
		{
			ID:          "compact",
			Title:       "Compact Session",
			Description: "Summarize the current session and create a new one with the summary",
			Handler: func(cmd Command) tea.Cmd {
				return func() tea.Msg {
					// TODO: implement compact message
					return ""
				}
			},
		},
	}
}

func (c *commandDialogCmp) ID() dialogs.DialogID {
	return commandsDialogID
}
