package commands

import (
	"github.com/charmbracelet/bubbles/v2/help"
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"github.com/opencode-ai/opencode/internal/tui/components/chat"
	"github.com/opencode-ai/opencode/internal/tui/components/completions"
	"github.com/opencode-ai/opencode/internal/tui/components/core"
	"github.com/opencode-ai/opencode/internal/tui/components/core/list"
	"github.com/opencode-ai/opencode/internal/tui/components/dialogs"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

const (
	CommandsDialogID dialogs.DialogID = "commands"

	defaultWidth int = 70
)

const (
	SystemCommands int = iota
	UserCommands
)

// Command represents a command that can be executed
type Command struct {
	ID          string
	Title       string
	Description string
	Shortcut    string // Optional shortcut for the command
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

	commandList  list.ListModel
	keyMap       CommandsDialogKeyMap
	help         help.Model
	commandType  int       // SystemCommands or UserCommands
	userCommands []Command // User-defined commands
}

type (
	SwitchSessionsMsg struct{}
	SwitchModelMsg    struct{}
)

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

	t := styles.CurrentTheme()
	commandList := list.New(
		list.WithFilterable(true),
		list.WithKeyMap(listKeyMap),
		list.WithWrapNavigation(true),
	)
	help := help.New()
	help.Styles = t.S().Help
	return &commandDialogCmp{
		commandList: commandList,
		width:       defaultWidth,
		keyMap:      DefaultCommandsDialogKeyMap(),
		help:        help,
		commandType: SystemCommands,
	}
}

func (c *commandDialogCmp) Init() tea.Cmd {
	commands, err := LoadCustomCommands()
	if err != nil {
		return util.ReportError(err)
	}

	c.userCommands = commands
	c.SetCommandType(c.commandType)
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
		case key.Matches(msg, c.keyMap.Tab):
			// Toggle command type between System and User commands
			if c.commandType == SystemCommands {
				return c, c.SetCommandType(UserCommands)
			} else {
				return c, c.SetCommandType(SystemCommands)
			}
		case key.Matches(msg, c.keyMap.Close):
			return c, util.CmdHandler(dialogs.CloseDialogMsg{})
		default:
			u, cmd := c.commandList.Update(msg)
			c.commandList = u.(list.ListModel)
			return c, cmd
		}
	}
	return c, nil
}

func (c *commandDialogCmp) View() tea.View {
	t := styles.CurrentTheme()
	listView := c.commandList.View()
	radio := c.commandTypeRadio()
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		t.S().Base.Padding(0, 1, 1, 1).Render(core.Title("Commands", c.width-lipgloss.Width(radio)-5)+" "+radio),
		listView.String(),
		"",
		t.S().Base.Width(c.width-2).PaddingLeft(1).AlignHorizontal(lipgloss.Left).Render(c.help.View(c.keyMap)),
	)
	v := tea.NewView(c.style().Render(content))
	if listView.Cursor() != nil {
		c := c.moveCursor(listView.Cursor())
		v.SetCursor(c)
	}
	return v
}

func (c *commandDialogCmp) commandTypeRadio() string {
	t := styles.CurrentTheme()
	choices := []string{"System", "User"}
	iconSelected := "◉"
	iconUnselected := "○"
	if c.commandType == SystemCommands {
		return t.S().Base.Foreground(t.FgHalfMuted).Render(iconSelected + " " + choices[0] + " " + iconUnselected + " " + choices[1])
	}
	return t.S().Base.Foreground(t.FgHalfMuted).Render(iconUnselected + " " + choices[0] + " " + iconSelected + " " + choices[1])
}

func (c *commandDialogCmp) listWidth() int {
	return defaultWidth - 2 // 4 for padding
}

func (c *commandDialogCmp) SetCommandType(commandType int) tea.Cmd {
	c.commandType = commandType

	var commands []Command
	if c.commandType == SystemCommands {
		commands = c.defaultCommands()
	} else {
		commands = c.userCommands
	}

	commandItems := []util.Model{}
	for _, cmd := range commands {
		opts := []completions.CompletionOption{}
		if cmd.Shortcut != "" {
			opts = append(opts, completions.WithShortcut(cmd.Shortcut))
		}
		commandItems = append(commandItems, completions.NewCompletionItem(cmd.Title, cmd, opts...))
	}
	return c.commandList.SetItems(commandItems)
}

func (c *commandDialogCmp) listHeight() int {
	listHeigh := len(c.commandList.Items()) + 2 + 4 // height based on items + 2 for the input + 4 for the sections
	return min(listHeigh, c.wHeight/2)
}

func (c *commandDialogCmp) moveCursor(cursor *tea.Cursor) *tea.Cursor {
	row, col := c.Position()
	offset := row + 3
	cursor.Y += offset
	cursor.X = cursor.X + col + 2
	return cursor
}

func (c *commandDialogCmp) style() lipgloss.Style {
	t := styles.CurrentTheme()
	return t.S().Base.
		Width(c.width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus)
}

func (c *commandDialogCmp) Position() (int, int) {
	row := c.wHeight/4 - 2 // just a bit above the center
	col := c.wWidth / 2
	col -= c.width / 2
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
		{
			ID:          "switch_session",
			Title:       "Switch Session",
			Description: "Switch to a different session",
			Shortcut:    "ctrl+s",
			Handler: func(cmd Command) tea.Cmd {
				return func() tea.Msg {
					return SwitchSessionsMsg{}
				}
			},
		},
		{
			ID:          "switch_model",
			Title:       "Switch Model",
			Description: "Switch to a different model",
			Handler: func(cmd Command) tea.Cmd {
				return func() tea.Msg {
					return SwitchModelMsg{}
				}
			},
		},
	}
}

func (c *commandDialogCmp) ID() dialogs.DialogID {
	return CommandsDialogID
}
