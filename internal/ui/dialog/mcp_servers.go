package dialog

import (
	"fmt"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/sahilm/fuzzy"
)

const (
	// MCPServersID is the identifier for the MCP servers dialog.
	MCPServersID              = "mcp_servers"
	mcpServersDialogMaxWidth  = 45
	mcpServersDialogMaxHeight = 12
	// rcGapHeight is the vertical gap (in lines) between the dialog title
	// and the list content.
	rcGapHeight = 1
)

// MCPServerItem represents a single MCP server in the list.
type MCPServerItem struct {
	*list.Versioned
	name      string
	state     mcp.State
	connected bool
	t         *styles.Styles
	m         fuzzy.Match
	cache     map[int]string
	focused   bool
}

// MCPServers represents a dialog for toggling MCP servers on/off.
type MCPServers struct {
	com  *common.Common
	help help.Model
	list *list.FilterableList

	keyMap struct {
		Toggle   key.Binding
		Next     key.Binding
		Previous key.Binding
		UpDown   key.Binding
		Close    key.Binding
	}
}

var (
	_ Dialog   = (*MCPServers)(nil)
	_ ListItem = (*MCPServerItem)(nil)
)

// NewMCPServers creates a new MCP servers dialog.
func NewMCPServers(com *common.Common) (*MCPServers, error) {
	r := &MCPServers{com: com}

	help := help.New()
	help.Styles = com.Styles.DialogHelpStyles()
	r.help = help

	r.list = list.NewFilterableList()
	r.list.Focus()

	r.keyMap.Toggle = key.NewBinding(
		key.WithKeys("space", "enter"),
		key.WithHelp("space", "toggle"),
	)
	r.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓", "next item"),
	)
	r.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", "previous item"),
	)
	r.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "choose"),
	)
	r.keyMap.Close = CloseKey

	if err := r.setMCPServerItems(); err != nil {
		return nil, err
	}

	return r, nil
}

// ID implements Dialog.
func (r *MCPServers) ID() string {
	return MCPServersID
}

// Refresh reloads MCP server states from the workspace and updates the list.
func (r *MCPServers) Refresh() {
	_ = r.setMCPServerItems()
}

// HandleMsg implements [Dialog].
func (r *MCPServers) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, r.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, r.keyMap.Previous):
			r.list.Focus()
			if r.list.IsSelectedFirst() {
				r.list.SelectLast()
				r.list.ScrollToBottom()
				break
			}
			r.list.SelectPrev()
			r.list.ScrollToSelected()
		case key.Matches(msg, r.keyMap.Next):
			r.list.Focus()
			if r.list.IsSelectedLast() {
				r.list.SelectFirst()
				r.list.ScrollToTop()
				break
			}
			r.list.SelectNext()
			r.list.ScrollToSelected()
		case key.Matches(msg, r.keyMap.Toggle):
			selectedItem := r.list.SelectedItem()
			if selectedItem == nil {
				break
			}
			mcpItem, ok := selectedItem.(*MCPServerItem)
			if !ok {
				break
			}
			// Ignore toggle while a server is starting to prevent
			// duplicate sessions.
			if mcpItem.state == mcp.StateStarting {
				break
			}
			return ActionToggleMCP{Name: mcpItem.name, Enable: !mcpItem.connected}
		}
	}
	return nil
}

// Cursor returns the cursor position relative to the dialog.
func (r *MCPServers) Cursor() *tea.Cursor {
	return nil
}

// Draw implements [Dialog].
func (r *MCPServers) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := r.com.Styles
	width := max(0, min(mcpServersDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(mcpServersDialogMaxHeight, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalBorderSize() + rcGapHeight

	r.list.SetSize(innerWidth, max(0, height-heightOffset))

	rc := NewRenderContext(t, width)
	rc.Title = "MCP Servers"
	rc.Gap = rcGapHeight

	visibleCount := len(r.list.FilteredItems())
	if r.list.Height() >= visibleCount {
		r.list.ScrollToTop()
	} else {
		r.list.ScrollToSelected()
	}

	listView := t.Dialog.List.Height(r.list.Height()).Render(r.list.Render())
	rc.AddPart(listView)
	rc.Help = renderDialogHelp(t, &r.help, r, innerWidth)

	view := rc.Render()
	DrawCenterCursor(scr, area, view, nil)
	return nil
}

// ShortHelp implements [help.KeyMap].
func (r *MCPServers) ShortHelp() []key.Binding {
	return []key.Binding{
		r.keyMap.UpDown,
		r.keyMap.Toggle,
		r.keyMap.Close,
	}
}

// FullHelp implements [help.KeyMap].
func (r *MCPServers) FullHelp() [][]key.Binding {
	m := [][]key.Binding{}
	slice := []key.Binding{
		r.keyMap.Toggle,
		r.keyMap.Next,
		r.keyMap.Previous,
		r.keyMap.Close,
	}
	for i := 0; i < len(slice); i += 4 {
		end := min(i+4, len(slice))
		m = append(m, slice[i:end])
	}
	return m
}

func (r *MCPServers) setMCPServerItems() error {
	cfg := r.com.Config()
	states := r.com.Workspace.MCPGetStates()

	var mcps []config.MCP
	mcps = append(mcps, cfg.MCP.Sorted()...)

	if len(mcps) == 0 {
		return fmt.Errorf("no MCP servers configured")
	}

	// Preserve current selection across refreshes.
	prevSelected := r.list.SelectedItem()
	var prevName string
	if prevSelected != nil {
		if item, ok := prevSelected.(*MCPServerItem); ok {
			prevName = item.name
		}
	}

	items := make([]list.FilterableItem, 0, len(mcps))
	selectedIndex := 0
	for i, mcpCfg := range mcps {
		state := mcp.StateDisabled
		connected := false
		if info, ok := states[mcpCfg.Name]; ok {
			state = info.State
			connected = info.State == mcp.StateConnected
		}

		item := &MCPServerItem{
			Versioned: list.NewVersioned(),
			name:      mcpCfg.Name,
			state:     state,
			connected: connected,
			t:         r.com.Styles,
		}
		items = append(items, item)
		if mcpCfg.Name == prevName {
			selectedIndex = i
		}
	}

	r.list.SetItems(items...)
	r.list.SetSelected(selectedIndex)
	r.list.ScrollToSelected()
	return nil
}

// Filter returns the filter value for the MCP server item.
func (i *MCPServerItem) Filter() string {
	return i.name
}

// ID returns the unique identifier for the MCP server item.
func (i *MCPServerItem) ID() string {
	return i.name
}

// SetFocused sets the focus state of the MCP server item.
func (i *MCPServerItem) SetFocused(focused bool) {
	if i.focused == focused {
		return
	}
	i.cache = nil
	i.focused = focused
	if i.Versioned != nil {
		i.Bump()
	}
}

// SetMatch sets the fuzzy match for the MCP server item.
func (i *MCPServerItem) SetMatch(m fuzzy.Match) {
	if sameFuzzyMatch(i.m, m) {
		return
	}
	i.cache = nil
	i.m = m
	if i.Versioned != nil {
		i.Bump()
	}
}

// Finished implements list.Item.
func (i *MCPServerItem) Finished() bool {
	return true
}

// Render returns the string representation of the MCP server item.
func (i *MCPServerItem) Render(width int) string {
	if i.cache == nil {
		i.cache = make(map[int]string)
	}

	cached, ok := i.cache[width]
	if ok {
		return cached
	}

	style := i.t.Dialog.NormalItem
	if i.focused {
		style = i.t.Dialog.SelectedItem
	}

	lineWidth := max(0, width-style.GetHorizontalFrameSize())

	// Build status indicator using the same colored dots as the sidebar.
	var iconStyle lipgloss.Style
	var statusText string
	switch i.state {
	case mcp.StateStarting:
		iconStyle = i.t.Resource.BusyIcon
		statusText = "starting"
	case mcp.StateConnected:
		iconStyle = i.t.Resource.OnlineIcon
		statusText = "on"
	case mcp.StateError:
		iconStyle = i.t.Resource.ErrorIcon
		statusText = "error"
	case mcp.StateDisabled:
		iconStyle = i.t.Resource.DisabledIcon
		statusText = "off"
	default:
		iconStyle = i.t.Resource.DisabledIcon
		statusText = "off"
	}
	toggleColor := iconStyle.GetForeground()
	toggleText := lipgloss.NewStyle().Foreground(toggleColor).Render(statusText + " ●")

	toggleWidth := lipgloss.Width(toggleText)

	// Display name (Docker MCP -> "Docker MCP", others -> config name).
	displayName := i.name
	if i.name == config.DockerMCPName {
		displayName = "Docker MCP"
	}

	title := lipgloss.NewStyle().MaxWidth(max(0, lineWidth-toggleWidth-1)).Render(displayName)
	titleWidth := lipgloss.Width(title)
	gap := lipgloss.NewStyle().Width(max(0, lineWidth-titleWidth-toggleWidth)).Render("")

	content := title + gap + toggleText
	result := style.Render(content)
	i.cache[width] = result
	return result
}
