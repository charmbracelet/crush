package commands

import (
	"os"

	"github.com/charmbracelet/bubbles/v2/help"
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/lipgloss/v2"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/tui/components/chat"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/exp/list"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
)

const (
	CommandsDialogID dialogs.DialogID = "commands"

	defaultWidth int = 70
)

const (
	SystemCommands int = iota
	UserCommands
)

type listModel = list.FilterableList[list.CompletionItem[Command]]

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

	commandList  listModel
	keyMap       CommandsDialogKeyMap
	help         help.Model
	commandType  int       // SystemCommands or UserCommands
	userCommands []Command // User-defined commands
	sessionID    string    // Current session ID
}

type (
	SwitchSessionsMsg      struct{}
	NewSessionsMsg         struct{}
	SwitchModelMsg         struct{}
	QuitMsg                struct{}
	OpenFilePickerMsg      struct{}
	ToggleHelpMsg          struct{}
	ToggleCompactModeMsg   struct{}
	ToggleThinkingMsg      struct{}
	OpenReasoningDialogMsg struct{}
	OpenExternalEditorMsg  struct{}
	ToggleYoloModeMsg      struct{}
	ReloadCommandsMsg      struct{}
	CompactMsg             struct {
		SessionID string
	}
)

func NewCommandDialog(sessionID string) CommandsDialog {
	keyMap := DefaultCommandsDialogKeyMap()
	listKeyMap := list.DefaultKeyMap()
	listKeyMap.Down.SetEnabled(false)
	listKeyMap.Up.SetEnabled(false)
	listKeyMap.DownOneItem = keyMap.Next
	listKeyMap.UpOneItem = keyMap.Previous

	t := styles.CurrentTheme()
	inputStyle := t.S().Base.PaddingLeft(1).PaddingBottom(1)
	commandList := list.NewFilterableList(
		[]list.CompletionItem[Command]{},
		list.WithFilterInputStyle(inputStyle),
		list.WithFilterListOptions(
			list.WithKeyMap(listKeyMap),
			list.WithWrapNavigation(),
			list.WithResizeByList(),
		),
	)
	help := help.New()
	help.Styles = t.S().Help
	return &commandDialogCmp{
		commandList: commandList,
		width:       defaultWidth,
		keyMap:      DefaultCommandsDialogKeyMap(),
		help:        help,
		commandType: SystemCommands,
		sessionID:   sessionID,
	}
}

func (c *commandDialogCmp) Init() tea.Cmd {
	commands, err := LoadCustomCommands()
	if err != nil {
		return util.ReportError(err)
	}
	c.userCommands = commands
	return c.SetCommandType(c.commandType)
}

func (c *commandDialogCmp) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		c.wWidth = msg.Width
		c.wHeight = msg.Height
		return c, tea.Batch(
			c.SetCommandType(c.commandType),
			c.commandList.SetSize(c.listWidth(), c.listHeight()),
		)
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, c.keyMap.Select):
			selectedItem := c.commandList.SelectedItem()
			if selectedItem == nil {
				return c, nil // No item selected, do nothing
			}
			command := (*selectedItem).Value()
			return c, tea.Sequence(
				util.CmdHandler(dialogs.CloseDialogMsg{}),
				command.Handler(command),
			)
		case key.Matches(msg, c.keyMap.Tab):
			if len(c.userCommands) == 0 {
				return c, nil
			}
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
			c.commandList = u.(listModel)
			return c, cmd
		}
	}
	return c, nil
}

func (c *commandDialogCmp) View() string {
	t := styles.CurrentTheme()
	listView := c.commandList
	radio := c.commandTypeRadio()

	header := t.S().Base.Padding(0, 1, 1, 1).Render(core.Title("Commands", c.width-lipgloss.Width(radio)-5) + " " + radio)
	if len(c.userCommands) == 0 {
		header = t.S().Base.Padding(0, 1, 1, 1).Render(core.Title("Commands", c.width-4))
	}
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		listView.View(),
		"",
		t.S().Base.Width(c.width-2).PaddingLeft(1).AlignHorizontal(lipgloss.Left).Render(c.help.View(c.keyMap)),
	)
	return c.style().Render(content)
}

func (c *commandDialogCmp) Cursor() *tea.Cursor {
	if cursor, ok := c.commandList.(util.Cursor); ok {
		cursor := cursor.Cursor()
		if cursor != nil {
			cursor = c.moveCursor(cursor)
		}
		return cursor
	}
	return nil
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

	commandItems := []list.CompletionItem[Command]{}
	for _, cmd := range commands {
		opts := []list.CompletionItemOption{
			list.WithCompletionID(cmd.ID),
		}
		if cmd.Shortcut != "" {
			opts = append(
				opts,
				list.WithCompletionShortcut(cmd.Shortcut),
			)
		}
		commandItems = append(commandItems, list.NewCompletionItem(cmd.Title, cmd, opts...))
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
	commands := []Command{
		{
			ID:          "new_session",
			Title:       "New Session",
			Description: "start a new session",
			Shortcut:    "ctrl+n",
			Handler: func(cmd Command) tea.Cmd {
				return util.CmdHandler(NewSessionsMsg{})
			},
		},
		{
			ID:          "switch_session",
			Title:       "Switch Session",
			Description: "Switch to a different session",
			Shortcut:    "ctrl+s",
			Handler: func(cmd Command) tea.Cmd {
				return util.CmdHandler(SwitchSessionsMsg{})
			},
		},
		{
			ID:          "switch_model",
			Title:       "Switch Model",
			Description: "Switch to a different model",
			Handler: func(cmd Command) tea.Cmd {
				return util.CmdHandler(SwitchModelMsg{})
			},
		},
	}

	// Only show compact command if there's an active session
	if c.sessionID != "" {
		commands = append(commands, Command{
			ID:          "Summarize",
			Title:       "Summarize Session",
			Description: "Summarize the current session and create a new one with the summary",
			Handler: func(cmd Command) tea.Cmd {
				return util.CmdHandler(CompactMsg{
					SessionID: c.sessionID,
				})
			},
		})
	}

	// Add reasoning toggle for models that support it
	cfg := config.Get()
	if agentCfg, ok := cfg.Agents[config.AgentCoder]; ok {
		providerCfg := cfg.GetProviderForModel(agentCfg.Model)
		model := cfg.GetModelByType(agentCfg.Model)
		if providerCfg != nil && model != nil && model.CanReason {
			selectedModel := cfg.Models[agentCfg.Model]

			// Anthropic models: thinking toggle
			if providerCfg.Type == catwalk.TypeAnthropic {
				status := "Enable"
				if selectedModel.Think {
					status = "Disable"
				}
				commands = append(commands, Command{
					ID:          "toggle_thinking",
					Title:       status + " Thinking Mode",
					Description: "Toggle model thinking for reasoning-capable models",
					Handler: func(cmd Command) tea.Cmd {
						return util.CmdHandler(ToggleThinkingMsg{})
					},
				})
			}

			// OpenAI models: reasoning effort dialog
			if len(model.ReasoningLevels) > 0 {
				commands = append(commands, Command{
					ID:          "select_reasoning_effort",
					Title:       "Select Reasoning Effort",
					Description: "Choose reasoning effort level (low/medium/high)",
					Handler: func(cmd Command) tea.Cmd {
						return util.CmdHandler(OpenReasoningDialogMsg{})
					},
				})
			}
		}
	}
	// Only show toggle compact mode command if window width is larger than compact breakpoint (90)
	if c.wWidth > 120 && c.sessionID != "" {
		commands = append(commands, Command{
			ID:          "toggle_sidebar",
			Title:       "Toggle Sidebar",
			Description: "Toggle between compact and normal layout",
			Handler: func(cmd Command) tea.Cmd {
				return util.CmdHandler(ToggleCompactModeMsg{})
			},
		})
	}
	if c.sessionID != "" {
		agentCfg := config.Get().Agents[config.AgentCoder]
		model := config.Get().GetModelByType(agentCfg.Model)
		if model.SupportsImages {
			commands = append(commands, Command{
				ID:          "file_picker",
				Title:       "Open File Picker",
				Shortcut:    "ctrl+f",
				Description: "Open file picker",
				Handler: func(cmd Command) tea.Cmd {
					return util.CmdHandler(OpenFilePickerMsg{})
				},
			})
		}
	}

	// Add external editor command if $EDITOR is available
	if os.Getenv("EDITOR") != "" {
		commands = append(commands, Command{
			ID:          "open_external_editor",
			Title:       "Open External Editor",
			Shortcut:    "ctrl+o",
			Description: "Open external editor to compose message",
			Handler: func(cmd Command) tea.Cmd {
				return util.CmdHandler(OpenExternalEditorMsg{})
			},
		})
	}

	return append(commands, []Command{
		{
			ID:          "toggle_yolo",
			Title:       "Toggle Yolo Mode",
			Description: "Toggle yolo mode",
			Handler: func(cmd Command) tea.Cmd {
				return util.CmdHandler(ToggleYoloModeMsg{})
			},
		},
		{
			ID:          "toggle_help",
			Title:       "Toggle Help",
			Shortcut:    "ctrl+g",
			Description: "Toggle help",
			Handler: func(cmd Command) tea.Cmd {
				return util.CmdHandler(ToggleHelpMsg{})
			},
		},
		{
			ID:          "reload_commands",
			Title:       "Reload Commands",
			Description: "Reload commands from all locations",
			Handler: func(cmd Command) tea.Cmd {
				return util.CmdHandler(ReloadCommandsMsg{})
			},
		},
		{
			ID:          "init",
			Title:       "Initialize Project",
			Description: "Create/Update the CRUSH.md memory file",
			Handler: func(cmd Command) tea.Cmd {
				return util.CmdHandler(chat.SendMsg{
					Text: agent.InitializePrompt(),
				})
			},
		},
		{
			ID:          "quit",
			Title:       "Quit",
			Description: "Quit",
			Shortcut:    "ctrl+c",
			Handler: func(cmd Command) tea.Cmd {
				return util.CmdHandler(QuitMsg{})
			},
		},
	}...)
}

func (c *commandDialogCmp) ID() dialogs.DialogID {
	return CommandsDialogID
}
