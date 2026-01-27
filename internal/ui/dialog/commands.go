package dialog

import (
	"os"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/agent/hyper"
	"github.com/charmbracelet/crush/internal/commands"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
)

// CommandsID is the identifier for the commands dialog.
const CommandsID = "commands"

// CommandType represents the type of commands being displayed.
type CommandType uint

// String returns the string representation of the CommandType.
func (c CommandType) String() string { return []string{"System", "User", "MCP"}[c] }

const (
	sidebarCompactModeBreakpoint  = 120
	defaultCommandsDialogMaxWidth = 70
)

const (
	SystemCommands CommandType = iota
	UserCommands
	MCPPrompts
)

// Commands represents a dialog that shows available commands.
type Commands struct {
	com    *common.Common
	keyMap struct {
		Select,
		UpDown,
		Next,
		Previous,
		Tab,
		ShiftTab,
		Close key.Binding
	}

	sessionID string // can be empty for non-session-specific commands
	selected  CommandType

	spinner spinner.Model
	loading bool

	help  help.Model
	input textinput.Model
	list  *list.FilterableList

	windowWidth int

	customCommands []commands.CustomCommand
	mcpPrompts     []commands.MCPPrompt
}

var _ Dialog = (*Commands)(nil)

// NewCommands creates a new commands dialog.
func NewCommands(com *common.Common, sessionID string, customCommands []commands.CustomCommand, mcpPrompts []commands.MCPPrompt) (*Commands, error) {
	c := &Commands{
		com:            com,
		selected:       SystemCommands,
		sessionID:      sessionID,
		customCommands: customCommands,
		mcpPrompts:     mcpPrompts,
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
		key.WithKeys("down"),
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
	c.keyMap.ShiftTab = key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("shift+tab", "switch selection prev"),
	)
	closeKey := CloseKey
	closeKey.SetHelp("esc", "cancel")
	c.keyMap.Close = closeKey

	// Set initial commands
	c.setCommandItems(c.selected)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = com.Styles.Dialog.Spinner
	c.spinner = s

	return c, nil
}

// ID implements Dialog.
func (c *Commands) ID() string {
	return CommandsID
}

// HandleMsg implements [Dialog].
func (c *Commands) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if c.loading {
			var cmd tea.Cmd
			c.spinner, cmd = c.spinner.Update(msg)
			return ActionCmd{Cmd: cmd}
		}
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
					return item.Action()
				}
			}
		case key.Matches(msg, c.keyMap.Tab):
			if len(c.customCommands) > 0 || len(c.mcpPrompts) > 0 {
				c.selected = c.nextCommandType()
				c.setCommandItems(c.selected)
			}
		case key.Matches(msg, c.keyMap.ShiftTab):
			if len(c.customCommands) > 0 || len(c.mcpPrompts) > 0 {
				c.selected = c.previousCommandType()
				c.setCommandItems(c.selected)
			}
		default:
			var cmd tea.Cmd
			for _, item := range c.list.FilteredItems() {
				if item, ok := item.(*CommandItem); ok && item != nil {
					if msg.String() == item.Shortcut() {
						return item.Action()
					}
				}
			}
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

// Cursor returns the cursor position relative to the dialog.
func (c *Commands) Cursor() *tea.Cursor {
	return InputCursor(c.com.Styles, c.input.Cursor())
}

// commandsRadioView generates the command type selector radio buttons.
func commandsRadioView(sty *styles.Styles, selected CommandType, hasUserCmds bool, hasMCPPrompts bool) string {
	if !hasUserCmds && !hasMCPPrompts {
		return ""
	}

	selectedFn := func(t CommandType) string {
		if t == selected {
			return sty.RadioOn.Padding(0, 1).Render() + sty.HalfMuted.Render(t.String())
		}
		return sty.RadioOff.Padding(0, 1).Render() + sty.HalfMuted.Render(t.String())
	}

	parts := []string{
		selectedFn(SystemCommands),
	}

	if hasUserCmds {
		parts = append(parts, selectedFn(UserCommands))
	}
	if hasMCPPrompts {
		parts = append(parts, selectedFn(MCPPrompts))
	}

	return strings.Join(parts, " ")
}

// Draw implements [Dialog].
func (c *Commands) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := c.com.Styles
	width := max(0, min(defaultCommandsDialogMaxWidth, area.Dx()))
	height := max(0, min(defaultDialogHeight, area.Dy()))
	if area.Dx() != c.windowWidth && c.selected == SystemCommands {
		c.windowWidth = area.Dx()
		// since some items in the list depend on width (e.g. toggle sidebar command),
		// we need to reset the command items when width changes
		c.setCommandItems(c.selected)
	}

	innerWidth := width - c.com.Styles.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.InputPrompt.GetVerticalFrameSize() + inputContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()

	c.input.SetWidth(max(0, innerWidth-t.Dialog.InputPrompt.GetHorizontalFrameSize()-1)) // (1) cursor padding

	c.list.SetSize(innerWidth, height-heightOffset)
	c.help.SetWidth(innerWidth)

	rc := NewRenderContext(t, width)
	rc.Title = "Commands"
	rc.TitleInfo = commandsRadioView(t, c.selected, len(c.customCommands) > 0, len(c.mcpPrompts) > 0)
	inputView := t.Dialog.InputPrompt.Render(c.input.View())
	rc.AddPart(inputView)
	listView := t.Dialog.List.Height(c.list.Height()).Render(c.list.Render())
	rc.AddPart(listView)
	rc.Help = c.help.View(c)

	if c.loading {
		rc.Help = c.spinner.View() + " Generating Prompt..."
	}

	view := rc.Render()

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

// nextCommandType returns the next command type in the cycle.
func (c *Commands) nextCommandType() CommandType {
	switch c.selected {
	case SystemCommands:
		if len(c.customCommands) > 0 {
			return UserCommands
		}
		if len(c.mcpPrompts) > 0 {
			return MCPPrompts
		}
		fallthrough
	case UserCommands:
		if len(c.mcpPrompts) > 0 {
			return MCPPrompts
		}
		fallthrough
	case MCPPrompts:
		return SystemCommands
	default:
		return SystemCommands
	}
}

// previousCommandType returns the previous command type in the cycle.
func (c *Commands) previousCommandType() CommandType {
	switch c.selected {
	case SystemCommands:
		if len(c.mcpPrompts) > 0 {
			return MCPPrompts
		}
		if len(c.customCommands) > 0 {
			return UserCommands
		}
		return SystemCommands
	case UserCommands:
		return SystemCommands
	case MCPPrompts:
		if len(c.customCommands) > 0 {
			return UserCommands
		}
		return SystemCommands
	default:
		return SystemCommands
	}
}

// setCommandItems sets the command items based on the specified command type.
func (c *Commands) setCommandItems(commandType CommandType) {
	c.selected = commandType

	commandItems := []list.FilterableItem{}
	switch c.selected {
	case SystemCommands:
		for _, cmd := range c.defaultCommands() {
			commandItems = append(commandItems, cmd)
		}
	case UserCommands:
		for _, cmd := range c.customCommands {
			action := ActionRunCustomCommand{
				Content:   cmd.Content,
				Arguments: cmd.Arguments,
			}
			commandItems = append(commandItems, NewCommandItem(c.com.Styles, "custom_"+cmd.ID, cmd.Name, "", action))
		}
	case MCPPrompts:
		for _, cmd := range c.mcpPrompts {
			action := ActionRunMCPPrompt{
				Title:       cmd.Title,
				Description: cmd.Description,
				PromptID:    cmd.PromptID,
				ClientID:    cmd.ClientID,
				Arguments:   cmd.Arguments,
			}
			commandItems = append(commandItems, NewCommandItem(c.com.Styles, "mcp_"+cmd.ID, cmd.PromptID, "", action))
		}
	}

	c.list.SetItems(commandItems...)
	c.list.SetFilter("")
	c.list.ScrollToTop()
	c.list.SetSelected(0)
	c.input.SetValue("")
}

// defaultCommands returns the list of default system commands.
func (c *Commands) defaultCommands() []*CommandItem {
	commands := []*CommandItem{
		NewCommandItem(c.com.Styles, "new_session", "New Session", "ctrl+n", ActionNewSession{}),
		NewCommandItem(c.com.Styles, "switch_session", "Sessions", "ctrl+s", ActionOpenDialog{SessionsID}),
		NewCommandItem(c.com.Styles, "switch_model", "Switch Model", "ctrl+l", ActionOpenDialog{ModelsID}),
	}

	// Only show compact command if there's an active session
	if c.sessionID != "" {
		commands = append(commands, NewCommandItem(c.com.Styles, "summarize", "Summarize Session", "", ActionSummarize{SessionID: c.sessionID}))
	}

	// Add reasoning toggle for models that support it
	cfg := c.com.Config()
	if agentCfg, ok := cfg.Agents[config.AgentCoder]; ok {
		providerCfg := cfg.GetProviderForModel(agentCfg.Model)
		model := cfg.GetModelByType(agentCfg.Model)
		if providerCfg != nil && model != nil && model.CanReason {
			selectedModel := cfg.Models[agentCfg.Model]

			// Anthropic models: thinking toggle
			if providerCfg.Type == catwalk.TypeAnthropic || providerCfg.Type == catwalk.Type(hyper.Name) {
				status := "Enable"
				if selectedModel.Think {
					status = "Disable"
				}
				commands = append(commands, NewCommandItem(c.com.Styles, "toggle_thinking", status+" Thinking Mode", "", ActionToggleThinking{}))
			}

			// OpenAI models: reasoning effort dialog
			if len(model.ReasoningLevels) > 0 {
				commands = append(commands, NewCommandItem(c.com.Styles, "select_reasoning_effort", "Select Reasoning Effort", "", ActionOpenDialog{
					DialogID: ReasoningID,
				}))
			}
		}
	}
	// Only show toggle compact mode command if window width is larger than compact breakpoint (120)
	if c.windowWidth >= sidebarCompactModeBreakpoint && c.sessionID != "" {
		commands = append(commands, NewCommandItem(c.com.Styles, "toggle_sidebar", "Toggle Sidebar", "", ActionToggleCompactMode{}))
	}
	if c.sessionID != "" {
		cfg := c.com.Config()
		agentCfg := cfg.Agents[config.AgentCoder]
		model := cfg.GetModelByType(agentCfg.Model)
		if model != nil && model.SupportsImages {
			commands = append(commands, NewCommandItem(c.com.Styles, "file_picker", "Open File Picker", "ctrl+f", ActionOpenDialog{
				// TODO: Pass in the file picker dialog id
			}))
		}
	}

	// Add external editor command if $EDITOR is available
	// TODO: Use [tea.EnvMsg] to get environment variable instead of os.Getenv
	if os.Getenv("EDITOR") != "" {
		commands = append(commands, NewCommandItem(c.com.Styles, "open_external_editor", "Open External Editor", "ctrl+o", ActionExternalEditor{}))
	}

	return append(commands,
		NewCommandItem(c.com.Styles, "toggle_yolo", "Toggle Yolo Mode", "", ActionToggleYoloMode{}),
		NewCommandItem(c.com.Styles, "toggle_help", "Toggle Help", "ctrl+g", ActionToggleHelp{}),
		NewCommandItem(c.com.Styles, "init", "Initialize Project", "", ActionInitializeProject{}),
		NewCommandItem(c.com.Styles, "quit", "Quit", "ctrl+c", tea.QuitMsg{}),
	)
}

// SetCustomCommands sets the custom commands and refreshes the view if user commands are currently displayed.
func (c *Commands) SetCustomCommands(customCommands []commands.CustomCommand) {
	c.customCommands = customCommands
	if c.selected == UserCommands {
		c.setCommandItems(c.selected)
	}
}

// SetMCPPrompts sets the MCP prompts and refreshes the view if MCP prompts are currently displayed.
func (c *Commands) SetMCPPrompts(mcpPrompts []commands.MCPPrompt) {
	c.mcpPrompts = mcpPrompts
	if c.selected == MCPPrompts {
		c.setCommandItems(c.selected)
	}
}

// StartLoading implements [LoadingDialog].
func (a *Commands) StartLoading() tea.Cmd {
	if a.loading {
		return nil
	}
	a.loading = true
	return a.spinner.Tick
}

// StopLoading implements [LoadingDialog].
func (a *Commands) StopLoading() {
	a.loading = false
}
