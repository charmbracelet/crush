package dialog

import (
	"os"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/commands"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
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
	sidebarCompactModeBreakpoint = 120
	// commandsSpareRows is the breathing room kept below the commands so the
	// list does not end flush against the help line.
	commandsSpareRows = 2
	// commandsMaxHeight caps the palette at the standard dialog height plus
	// its spare rows; longer command lists scroll.
	commandsMaxHeight = defaultDialogHeight + commandsSpareRows
)

const (
	SystemCommands CommandType = iota
	UserCommands
	MCPPrompts
)

// Commands represents a dialog that shows available commands.
type dockerMCPAvailabilityCheckedMsg struct {
	available bool
}

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

	sessionID  string
	hasSession bool
	hasTodos   bool
	hasQueue   bool
	selected   CommandType

	spinner spinner.Model
	loading bool

	help  help.Model
	input textinput.Model
	list  *CommandsList

	windowWidth int

	customCommands []commands.CustomCommand
	mcpPrompts     []commands.MCPPrompt

	// Items for every tab, kept so the dialog can measure the tallest one
	// instead of only the tab on screen.
	systemGroups []CommandGroup
	userItems    []*CommandItem
	mcpItems     []*CommandItem

	dockerMCPAvailable     *bool
	dockerMCPCheckInFlight bool
}

var _ Dialog = (*Commands)(nil)

// NewCommands creates a new commands dialog.
func NewCommands(com *common.Common, sessionID string, hasSession, hasTodos, hasQueue bool, customCommands []commands.CustomCommand, mcpPrompts []commands.MCPPrompt) (*Commands, error) {
	c := &Commands{
		com:            com,
		selected:       SystemCommands,
		sessionID:      sessionID,
		hasSession:     hasSession,
		hasTodos:       hasTodos,
		hasQueue:       hasQueue,
		customCommands: customCommands,
		mcpPrompts:     mcpPrompts,
	}

	help := help.New()
	help.Styles = com.Styles.DialogHelpStyles()

	c.help = help

	c.list = NewCommandsList(com.Styles)
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

	if available, known := config.DockerMCPAvailabilityCached(); known {
		c.dockerMCPAvailable = &available
	}

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
	case dockerMCPAvailabilityCheckedMsg:
		c.dockerMCPAvailable = &msg.available
		c.dockerMCPCheckInFlight = false
		if c.selected == SystemCommands {
			// Preserve the current selection across the rebuild to avoid reset
			var prevID string
			if item, ok := c.list.SelectedItem().(*CommandItem); ok && item != nil {
				prevID = item.id
			}
			c.setCommandItems(c.selected)
			if prevID != "" {
				for i, it := range c.list.VisibleItems() {
					if ci, ok := it.(*CommandItem); ok && ci != nil && ci.id == prevID {
						c.list.SetSelected(i)
						c.list.ScrollToSelected()
						break
					}
				}
			}
		}
		return nil
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
			} else {
				c.list.SelectPrev()
			}
			c.list.ScrollToSelected()
		case key.Matches(msg, c.keyMap.Next):
			c.list.Focus()
			if c.list.IsSelectedLast() {
				c.list.SelectFirst()
			} else {
				c.list.SelectNext()
			}
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
			for _, item := range commandItems(c.list.VisibleItems()) {
				if msg.String() == item.Shortcut() {
					return item.Action()
				}
			}
			prevValue := c.input.Value()
			c.input, cmd = c.input.Update(msg)
			value := c.input.Value()
			if value != prevValue {
				c.list.SetFilter(value)
				c.list.ScrollToTop()
				c.list.SetSelected(0)
			}
			return ActionCmd{cmd}
		}
	}
	return nil
}

func checkDockerMCPAvailabilityCmd() tea.Cmd {
	return func() tea.Msg {
		return dockerMCPAvailabilityCheckedMsg{available: config.RefreshDockerMCPAvailability()}
	}
}

func (c *Commands) InitialCmd() tea.Cmd {
	if c.dockerMCPAvailable != nil || c.dockerMCPCheckInFlight {
		return nil
	}
	c.dockerMCPCheckInFlight = true
	return checkDockerMCPAvailabilityCmd()
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
			return sty.Radio.On.Padding(0, 1).Render() + sty.Radio.Label.Render(t.String())
		}
		return sty.Radio.Off.Padding(0, 1).Render() + sty.Radio.Label.Render(t.String())
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
	width := max(0, min(defaultDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	if area.Dx() != c.windowWidth && c.selected == SystemCommands {
		c.windowWidth = area.Dx()
		// since some items in the list depend on width (e.g. toggle sidebar command),
		// we need to reset the command items when width changes
		c.setCommandItems(c.selected)
	}

	innerWidth := width - c.com.Styles.Dialog.View.GetHorizontalFrameSize()
	c.input.SetWidth(dialogInputTextWidth(t, c.input, innerWidth))

	// Size to the tallest tab plus a couple of spare rows so the palette
	// keeps one height as tabs and filters change, bounded by the dialog
	// maximum and by what the screen has room for.
	height := min(
		fitDialogHeight(t, c.tallestTabHeight(innerWidth), commandsSpareRows, area.Dy()),
		commandsMaxHeight,
	)

	listHeight, listTotalHeight, listWidth := sizeDialogList(t, c.list, innerWidth, height)

	// Hide the shortcut hints uniformly when the widest would crowd names.
	applyInfoColumnVisibility(c.list.VisibleItems(), listWidth, commandInfoMaxPercent)

	rc := NewRenderContext(t, width)
	rc.Title = "Commands"
	rc.TitleInfo = commandsRadioView(t, c.selected, len(c.customCommands) > 0, len(c.mcpPrompts) > 0)
	inputView := t.Dialog.InputPrompt.Render(c.input.View())
	rc.AddPart(inputView)
	listView := t.Dialog.List.Height(c.list.Height()).Render(c.list.Render())
	listView = joinScrollbar(t, listView, listHeight, listTotalHeight, listHeight, c.list.Offset())
	rc.AddPart(listView)
	rc.Help = renderDialogHelp(t, &c.help, c, innerWidth)

	if c.loading {
		rc.Help = t.Dialog.HelpView.Width(innerWidth).Render(c.spinner.View() + " Generating Prompt...")
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
// Every tab's items are built, not just the visible one, so the dialog can
// size itself to the tallest tab and keep that height as tabs change.
func (c *Commands) setCommandItems(commandType CommandType) {
	c.selected = commandType

	c.systemGroups = c.defaultCommandGroups()
	c.userItems = c.customCommandItems()
	c.mcpItems = c.mcpPromptItems()

	switch c.selected {
	case SystemCommands:
		c.list.SetGroups(c.systemGroups...)
	case UserCommands:
		c.list.SetItems(c.userItems...)
	case MCPPrompts:
		c.list.SetItems(c.mcpItems...)
	}

	c.list.SetFilter("")
	c.list.ScrollToTop()
	c.list.SetSelected(0)
	c.input.SetValue("")
}

// customCommandItems returns the items for the user commands tab.
func (c *Commands) customCommandItems() []*CommandItem {
	items := []*CommandItem{}
	for _, cmd := range c.customCommands {
		var action Action
		if cmd.Skill != nil {
			action = ActionAttachSkill{ID: cmd.Skill.SkillFilePath, Name: cmd.Skill.Name}
		} else {
			action = ActionRunCustomCommand{
				Content:   cmd.Content,
				Arguments: cmd.Arguments,
				Skill:     cmd.Skill,
			}
		}
		item := NewCommandItem(c.com.Styles, "custom_"+cmd.ID, cmd.Name, "", action)
		if cmd.Skill != nil {
			item = item.WithDescription(cmd.Skill.Description)
		}
		items = append(items, item)
	}
	return items
}

// mcpPromptItems returns the items for the MCP prompts tab.
func (c *Commands) mcpPromptItems() []*CommandItem {
	items := []*CommandItem{}
	for _, cmd := range c.mcpPrompts {
		action := ActionRunMCPPrompt{
			Title:       cmd.Title,
			Description: cmd.Description,
			PromptID:    cmd.PromptID,
			ClientID:    cmd.ClientID,
			Arguments:   cmd.Arguments,
		}
		items = append(items, NewCommandItem(c.com.Styles, "mcp_"+cmd.ID, cmd.PromptID, "", action))
	}
	return items
}

// tallestTabHeight returns the content height of the tab with the most rows
// at the given width, so switching tabs never resizes the dialog.
func (c *Commands) tallestTabHeight(width int) int {
	return max(
		groupsContentHeight(width, c.systemGroups),
		itemsContentHeight(width, c.userItems),
		itemsContentHeight(width, c.mcpItems),
	)
}

// defaultCommandGroups returns the default system commands grouped into
// sections. Empty sections are omitted.
func (c *Commands) defaultCommandGroups() []CommandGroup {
	t := c.com.Styles

	session := []*CommandItem{
		NewCommandItem(t, "new_session", "New Session", "ctrl+n", ActionNewSession{}).WithAliases("clear"),
		NewCommandItem(t, "switch_session", "Open Session", "ctrl+s", ActionOpenDialog{SessionsID}),
	}

	// Only show compact command if there's an active session
	if c.hasSession {
		session = append(session, NewCommandItem(t, "summarize", "Summarize Session", "", ActionSummarize{SessionID: c.sessionID}))
	}

	if c.hasSession {
		cfgPrime := c.com.Config()
		agentCfg := cfgPrime.Agents[config.AgentCoder]
		model := cfgPrime.GetModelByType(agentCfg.Model)
		if model != nil && model.SupportsImages {
			session = append(session, NewCommandItem(t, "file_picker", "Open File Picker", "ctrl+f", ActionOpenDialog{
				DialogID: FilePickerID,
			}))
		}
	}

	session = append(
		session,
		NewCommandItem(t, "init", "Initialize Project", "", ActionInitializeProject{}),
		NewCommandItem(t, "toggle_yolo", "Toggle Yolo Mode", "ctrl+y", ActionToggleYoloMode{}),
	)

	model := []*CommandItem{
		NewCommandItem(t, "switch_model", "Switch Model", "ctrl+l", ActionOpenDialog{ModelsID}),
	}

	// Add reasoning toggle for models that support it
	cfg := c.com.Config()
	if agentCfg, ok := cfg.Agents[config.AgentCoder]; ok {
		providerCfg := cfg.GetProviderForModel(agentCfg.Model)
		reasoningModel := cfg.GetModelByType(agentCfg.Model)
		if providerCfg != nil && reasoningModel != nil && reasoningModel.CanReason {
			selectedModel := cfg.Models[agentCfg.Model]

			// Anthropic models: thinking toggle
			if reasoningModel.CanReason && len(reasoningModel.ReasoningLevels) == 0 {
				status := "Enable"
				if selectedModel.Think {
					status = "Disable"
				}
				model = append(model, NewCommandItem(t, "toggle_thinking", status+" Thinking Mode", "", ActionToggleThinking{}))
			}

			// OpenAI models: reasoning effort dialog
			if len(reasoningModel.ReasoningLevels) > 0 {
				model = append(model, NewCommandItem(t, "select_reasoning_effort", "Select Reasoning Effort", "", ActionOpenDialog{
					DialogID: ReasoningID,
				}))
			}
		}
	}

	settings := []*CommandItem{
		NewCommandItem(t, "switch_theme", "Themes", "", ActionOpenDialog{DialogID: ThemeID}),
	}

	// Only show toggle compact mode command if window width is larger than compact breakpoint (120)
	if c.windowWidth >= sidebarCompactModeBreakpoint && c.hasSession {
		settings = append(settings, NewCommandItem(t, "toggle_sidebar", "Toggle Sidebar", "", ActionToggleCompactMode{}))
	}

	// Add Docker MCP command if available and not already enabled.
	if !cfg.IsDockerMCPEnabled() && c.dockerMCPAvailable != nil && *c.dockerMCPAvailable {
		settings = append(settings, NewCommandItem(t, "enable_docker_mcp", "Enable Docker MCP Catalog", "", ActionEnableDockerMCP{}))
	}

	// Add disable Docker MCP command if it's currently enabled
	if cfg.IsDockerMCPEnabled() {
		settings = append(settings, NewCommandItem(t, "disable_docker_mcp", "Disable Docker MCP Catalog", "", ActionDisableDockerMCP{}))
	}

	if c.hasTodos || c.hasQueue {
		var label string
		switch {
		case c.hasTodos && c.hasQueue:
			label = "Toggle To-Dos/Queue"
		case c.hasQueue:
			label = "Toggle Queue"
		default:
			label = "Toggle To-Dos"
		}
		settings = append(settings, NewCommandItem(t, "toggle_pills", label, "ctrl+t", ActionTogglePills{}))
	}

	// Add a command for selecting notification style via picker dialog.
	notificationLabel := "Notification Style"
	settings = append(settings, NewCommandItem(t, "select_notifications", notificationLabel, "", ActionOpenDialog{DialogID: NotificationsID}))

	// Add transparent background toggle.
	transparentLabel := "Disable Background Color"
	if cfg != nil && cfg.Options != nil && cfg.Options.TUI.IsTransparent() {
		transparentLabel = "Enable Background Color"
	}
	settings = append(settings, NewCommandItem(t, "toggle_transparent", transparentLabel, "", ActionToggleTransparentBackground{}))

	// Add mouse support toggle.
	mouseLabel := "Disable Mouse"
	if cfg != nil && cfg.Options != nil && cfg.Options.TUI.Mouse != nil && !*cfg.Options.TUI.Mouse {
		mouseLabel = "Enable Mouse"
	}
	settings = append(settings, NewCommandItem(t, "toggle_mouse", mouseLabel, "", ActionToggleMouseSupport{}))

	application := []*CommandItem{}

	// Add external editor command if $EDITOR is available.
	//
	// TODO: Use [tea.EnvMsg] to get environment variable instead of os.Getenv;
	// because os.Getenv does IO is breaks the TEA paradigm and is generally an
	// antipattern.
	if os.Getenv("EDITOR") != "" {
		application = append(application, NewCommandItem(t, "open_external_editor", "Open External Editor", "ctrl+o", ActionExternalEditor{}))
	}

	application = append(
		application,
		NewCommandItem(t, "toggle_help", "Toggle Help", "ctrl+g", ActionToggleHelp{}),
		NewCommandItem(t, "quit", "Quit", "ctrl+c", tea.QuitMsg{}).WithAliases("exit"),
	)

	groups := []CommandGroup{}
	addGroup := func(title string, items []*CommandItem) {
		if len(items) > 0 {
			groups = append(groups, NewCommandGroup(t, title, items...))
		}
	}
	addGroup("Sessions", session)
	addGroup("Models", model)
	addGroup("Settings", settings)
	addGroup("Application", application)
	return groups
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
func (c *Commands) StartLoading() tea.Cmd {
	if c.loading {
		return nil
	}
	c.loading = true
	return c.spinner.Tick
}

// StopLoading implements [LoadingDialog].
func (c *Commands) StopLoading() {
	c.loading = false
}
