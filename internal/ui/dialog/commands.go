package dialog

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/uicmd"
	"github.com/charmbracelet/crush/internal/uiutil"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// CommandsID is the identifier for the commands dialog.
const CommandsID = "commands"

// Commands represents a dialog that shows available commands.
type Commands struct {
	com    *common.Common
	keyMap struct {
		Select,
		UpDown,
		Next,
		Previous,
		Tab,
		Close key.Binding
	}

	sessionID  string // can be empty for non-session-specific commands
	selected   uicmd.CommandType
	userCmds   []uicmd.Command
	mcpPrompts *csync.Slice[uicmd.Command]

	help  help.Model
	input textinput.Model
	list  *list.FilterableList

	width int
}

var _ Dialog = (*Commands)(nil)

// NewCommands creates a new commands dialog.
func NewCommands(com *common.Common, sessionID string) (*Commands, error) {
	commands, err := uicmd.LoadCustomCommandsFromConfig(com.Config())
	if err != nil {
		return nil, err
	}

	mcpPrompts := csync.NewSlice[uicmd.Command]()
	mcpPrompts.SetSlice(uicmd.LoadMCPPrompts())

	c := &Commands{
		com:        com,
		userCmds:   commands,
		selected:   uicmd.SystemCommands,
		mcpPrompts: mcpPrompts,
		sessionID:  sessionID,
	}

	help := help.New()
	help.Styles = com.Styles.DialogHelpStyles()

	c.help = help

	c.list = list.NewFilterableList()
	c.list.Focus()
	c.list.SetSelected(0)

	c.input = textinput.New()
	c.input.SetVirtualCursor(false)
	c.input.Placeholder = "Type to filter"
	c.input.SetStyles(com.Styles.TextInput)
	c.input.Focus()

	c.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "confirm"),
	)
	c.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "choose"),
	)
	c.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓", "next item"),
	)
	c.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", "previous item"),
	)
	c.keyMap.Tab = key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch selection"),
	)
	closeKey := CloseKey
	closeKey.SetHelp("esc", "cancel")
	c.keyMap.Close = closeKey

	// Set initial commands
	c.setCommandType(c.selected)

	return c, nil
}

// ID implements Dialog.
func (c *Commands) ID() string {
	return CommandsID
}

// HandleMsg implements Dialog.
func (c *Commands) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, c.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, c.keyMap.Previous):
			c.list.Focus()
			if c.list.IsSelectedFirst() {
				c.list.SelectLast()
				c.list.ScrollToBottom()
				break
			}
			c.list.SelectPrev()
			c.list.ScrollToSelected()
		case key.Matches(msg, c.keyMap.Next):
			c.list.Focus()
			if c.list.IsSelectedLast() {
				c.list.SelectFirst()
				c.list.ScrollToTop()
				break
			}
			c.list.SelectNext()
			c.list.ScrollToSelected()
		case key.Matches(msg, c.keyMap.Select):
			if selectedItem := c.list.SelectedItem(); selectedItem != nil {
				if item, ok := selectedItem.(*CommandItem); ok && item != nil {
					// TODO: Please unravel this mess later and the Command
					// Handler design.
					if cmd := item.Cmd.Handler(item.Cmd); cmd != nil { // Huh??
						return cmd()
					}
				}
			}
		case key.Matches(msg, c.keyMap.Tab):
			if len(c.userCmds) > 0 || c.mcpPrompts.Len() > 0 {
				c.selected = c.nextCommandType()
				c.setCommandType(c.selected)
			}
		default:
			var cmd tea.Cmd
			c.input, cmd = c.input.Update(msg)
			value := c.input.Value()
			c.list.SetFilter(value)
			c.list.ScrollToTop()
			c.list.SetSelected(0)
			return ActionCmd{cmd}
		}
	}
	return nil
}

// ReloadMCPPrompts reloads the MCP prompts.
func (c *Commands) ReloadMCPPrompts() tea.Cmd {
	c.mcpPrompts.SetSlice(uicmd.LoadMCPPrompts())
	// If we're currently viewing MCP prompts, refresh the list
	if c.selected == uicmd.MCPPrompts {
		c.setCommandType(uicmd.MCPPrompts)
	}
	return nil
}

// Cursor returns the cursor position relative to the dialog.
func (c *Commands) Cursor() *tea.Cursor {
	return InputCursor(c.com.Styles, c.input.Cursor())
}

// commandsRadioView generates the command type selector radio buttons.
func commandsRadioView(sty *styles.Styles, selected uicmd.CommandType, hasUserCmds bool, hasMCPPrompts bool) string {
	if !hasUserCmds && !hasMCPPrompts {
		return ""
	}

	selectedFn := func(t uicmd.CommandType) string {
		if t == selected {
			return sty.RadioOn.Padding(0, 1).Render() + sty.HalfMuted.Render(t.String())
		}
		return sty.RadioOff.Padding(0, 1).Render() + sty.HalfMuted.Render(t.String())
	}

	parts := []string{
		selectedFn(uicmd.SystemCommands),
	}

	if hasUserCmds {
		parts = append(parts, selectedFn(uicmd.UserCommands))
	}
	if hasMCPPrompts {
		parts = append(parts, selectedFn(uicmd.MCPPrompts))
	}

	return strings.Join(parts, " ")
}

// Draw implements [Dialog].
func (c *Commands) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := c.com.Styles
	width := max(0, min(100, area.Dx()))
	height := max(0, min(30, area.Dy()))
	c.width = width
	// TODO: Why do we need this 2?
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize() - 2
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + 1 + // (1) title content
		t.Dialog.InputPrompt.GetVerticalFrameSize() + 1 + // (1) input content
		t.Dialog.HelpView.GetVerticalFrameSize() +
		// TODO: Why do we need this 2?
		t.Dialog.View.GetVerticalFrameSize() + 2
	c.input.SetWidth(innerWidth - t.Dialog.InputPrompt.GetHorizontalFrameSize() - 1) // (1) cursor padding
	c.list.SetSize(innerWidth, height-heightOffset)
	c.help.SetWidth(innerWidth)

	radio := commandsRadioView(t, c.selected, len(c.userCmds) > 0, c.mcpPrompts.Len() > 0)
	titleStyle := t.Dialog.Title
	dialogStyle := t.Dialog.View.Width(width)
	headerOffset := lipgloss.Width(radio) + titleStyle.GetHorizontalFrameSize() + dialogStyle.GetHorizontalFrameSize()
	helpView := ansi.Truncate(c.help.View(c), innerWidth, "")
	header := common.DialogTitle(t, "Commands", width-headerOffset) + radio
	view := HeaderInputListHelpView(t, width, c.list.Height(), header,
		c.input.View(), c.list.Render(), helpView)

	cur := c.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

// ShortHelp implements [help.KeyMap].
func (c *Commands) ShortHelp() []key.Binding {
	return []key.Binding{
		c.keyMap.Tab,
		c.keyMap.UpDown,
		c.keyMap.Select,
		c.keyMap.Close,
	}
}

// FullHelp implements [help.KeyMap].
func (c *Commands) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{c.keyMap.Select, c.keyMap.Next, c.keyMap.Previous, c.keyMap.Tab},
		{c.keyMap.Close},
	}
}

func (c *Commands) nextCommandType() uicmd.CommandType {
	switch c.selected {
	case uicmd.SystemCommands:
		if len(c.userCmds) > 0 {
			return uicmd.UserCommands
		}
		if c.mcpPrompts.Len() > 0 {
			return uicmd.MCPPrompts
		}
		fallthrough
	case uicmd.UserCommands:
		if c.mcpPrompts.Len() > 0 {
			return uicmd.MCPPrompts
		}
		fallthrough
	case uicmd.MCPPrompts:
		return uicmd.SystemCommands
	default:
		return uicmd.SystemCommands
	}
}

func (c *Commands) setCommandType(commandType uicmd.CommandType) {
	c.selected = commandType

	var commands []uicmd.Command
	switch c.selected {
	case uicmd.SystemCommands:
		commands = c.defaultCommands()
	case uicmd.UserCommands:
		commands = c.userCmds
	case uicmd.MCPPrompts:
		commands = slices.Collect(c.mcpPrompts.Seq())
	}

	commandItems := []list.FilterableItem{}
	for _, cmd := range commands {
		commandItems = append(commandItems, NewCommandItem(c.com.Styles, cmd))
	}

	c.list.SetItems(commandItems...)
	c.list.SetSelected(0)
	c.list.SetFilter("")
	c.list.ScrollToTop()
	c.list.SetSelected(0)
	c.input.SetValue("")
}

// TODO: Rethink this
func (c *Commands) defaultCommands() []uicmd.Command {
	commands := []uicmd.Command{
		{
			ID:          "new_session",
			Title:       "New Session",
			Description: "start a new session",
			Shortcut:    "ctrl+n",
			Handler: func(cmd uicmd.Command) tea.Cmd {
				return uiutil.CmdHandler(ActionNewSession{})
			},
		},
		{
			ID:          "switch_session",
			Title:       "Switch Session",
			Description: "Switch to a different session",
			Shortcut:    "ctrl+s",
			Handler: func(cmd uicmd.Command) tea.Cmd {
				return uiutil.CmdHandler(ActionOpenDialog{SessionsID})
			},
		},
		{
			ID:          "switch_model",
			Title:       "Switch Model",
			Description: "Switch to a different model",
			// FIXME: The shortcut might get updated if enhanced keyboard is supported.
			Shortcut: "ctrl+l",
			Handler: func(cmd uicmd.Command) tea.Cmd {
				return uiutil.CmdHandler(ActionOpenDialog{ModelsID})
			},
		},
	}

	// Only show compact command if there's an active session
	if c.sessionID != "" {
		commands = append(commands, uicmd.Command{
			ID:          "Summarize",
			Title:       "Summarize Session",
			Description: "Summarize the current session and create a new one with the summary",
			Handler: func(cmd uicmd.Command) tea.Cmd {
				return uiutil.CmdHandler(ActionSummarize{
					SessionID: c.sessionID,
				})
			},
		})
	}

	// Add reasoning toggle for models that support it
	cfg := c.com.Config()
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
				commands = append(commands, uicmd.Command{
					ID:          "toggle_thinking",
					Title:       status + " Thinking Mode",
					Description: "Toggle model thinking for reasoning-capable models",
					Handler: func(cmd uicmd.Command) tea.Cmd {
						return uiutil.CmdHandler(ActionToggleThinking{})
					},
				})
			}

			// OpenAI models: reasoning effort dialog
			if len(model.ReasoningLevels) > 0 {
				commands = append(commands, uicmd.Command{
					ID:          "select_reasoning_effort",
					Title:       "Select Reasoning Effort",
					Description: "Choose reasoning effort level (low/medium/high)",
					Handler: func(cmd uicmd.Command) tea.Cmd {
						return uiutil.CmdHandler(ActionOpenDialog{
							// TODO: Pass reasoning dialog id
						})
					},
				})
			}
		}
	}
	// Only show toggle compact mode command if window width is larger than compact breakpoint (90)
	// TODO: Get. Rid. Of. Magic. Numbers!
	if c.width > 120 && c.sessionID != "" {
		commands = append(commands, uicmd.Command{
			ID:          "toggle_sidebar",
			Title:       "Toggle Sidebar",
			Description: "Toggle between compact and normal layout",
			Handler: func(cmd uicmd.Command) tea.Cmd {
				return uiutil.CmdHandler(ActionToggleCompactMode{})
			},
		})
	}
	if c.sessionID != "" {
		cfg := c.com.Config()
		agentCfg := cfg.Agents[config.AgentCoder]
		model := cfg.GetModelByType(agentCfg.Model)
		if model.SupportsImages {
			commands = append(commands, uicmd.Command{
				ID:          "file_picker",
				Title:       "Open File Picker",
				Shortcut:    "ctrl+f",
				Description: "Open file picker",
				Handler: func(cmd uicmd.Command) tea.Cmd {
					return uiutil.CmdHandler(ActionOpenDialog{
						// TODO: Pass file picker dialog id
					})
				},
			})
		}
	}

	// Add external editor command if $EDITOR is available
	// TODO: Use [tea.EnvMsg] to get environment variable instead of os.Getenv
	if os.Getenv("EDITOR") != "" {
		commands = append(commands, uicmd.Command{
			ID:          "open_external_editor",
			Title:       "Open External Editor",
			Shortcut:    "ctrl+o",
			Description: "Open external editor to compose message",
			Handler: func(cmd uicmd.Command) tea.Cmd {
				return uiutil.CmdHandler(ActionExternalEditor{})
			},
		})
	}

	return append(commands, []uicmd.Command{
		{
			ID:          "toggle_yolo",
			Title:       "Toggle Yolo Mode",
			Description: "Toggle yolo mode",
			Handler: func(cmd uicmd.Command) tea.Cmd {
				return uiutil.CmdHandler(ActionToggleYoloMode{})
			},
		},
		{
			ID:          "toggle_help",
			Title:       "Toggle Help",
			Shortcut:    "ctrl+g",
			Description: "Toggle help",
			Handler: func(cmd uicmd.Command) tea.Cmd {
				return uiutil.CmdHandler(ActionToggleHelp{})
			},
		},
		{
			ID:          "init",
			Title:       "Initialize Project",
			Description: fmt.Sprintf("Create/Update the %s memory file", config.Get().Options.InitializeAs),
			Handler: func(cmd uicmd.Command) tea.Cmd {
				initPrompt, err := agent.InitializePrompt(*c.com.Config())
				if err != nil {
					return uiutil.ReportError(err)
				}
				return uiutil.CmdHandler(chat.SendMsg{
					Text: initPrompt,
				})
			},
		},
		{
			ID:          "quit",
			Title:       "Quit",
			Description: "Quit",
			Shortcut:    "ctrl+c",
			Handler: func(cmd uicmd.Command) tea.Cmd {
				return uiutil.CmdHandler(tea.QuitMsg{})
			},
		},
	}...)
}
