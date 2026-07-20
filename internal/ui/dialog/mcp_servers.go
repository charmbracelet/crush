package dialog

import (
	"fmt"
	"sort"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/sahilm/fuzzy"
)

// MCPServersID is the identifier for the MCP servers dialog.
const MCPServersID = "mcp_servers"

// MCPServerItem wraps an MCP server's ClientInfo as a filterable list item.
type MCPServerItem struct {
	*list.Versioned
	info    mcp.ClientInfo
	t       *styles.Styles
	m       fuzzy.Match
	cache   map[int]string
	focused bool
}

var _ ListItem = &MCPServerItem{Versioned: list.NewVersioned()}

// NewMCPServerItem creates a new MCPServerItem.
func NewMCPServerItem(t *styles.Styles, info mcp.ClientInfo) *MCPServerItem {
	return &MCPServerItem{
		Versioned: list.NewVersioned(),
		t:         t,
		info:      info,
	}
}

// Finished implements list.Item.
func (m *MCPServerItem) Finished() bool { return true }

// Filter implements ListItem.
func (m *MCPServerItem) Filter() string {
	return m.info.Name
}

// ID implements ListItem.
func (m *MCPServerItem) ID() string {
	return m.info.Name
}

// SetFocused implements ListItem.
func (m *MCPServerItem) SetFocused(focused bool) {
	if m.focused == focused {
		return
	}
	m.cache = nil
	m.focused = focused
	if m.Versioned != nil {
		m.Bump()
	}
}

// SetMatch implements ListItem.
func (m *MCPServerItem) SetMatch(match fuzzy.Match) {
	if sameFuzzyMatch(m.m, match) {
		return
	}
	m.cache = nil
	m.m = match
	if m.Versioned != nil {
		m.Bump()
	}
}

// Render implements ListItem.
func (m *MCPServerItem) Render(width int) string {
	itemStyles := ListItemStyles{
		ItemBlurred:     m.t.Dialog.NormalItem,
		ItemFocused:     m.t.Dialog.SelectedItem,
		InfoTextBlurred: m.t.Dialog.ListItem.InfoBlurred,
		InfoTextFocused: m.t.Dialog.ListItem.InfoFocused,
	}

	info := fmt.Sprintf("%s  %dt %dp %dr",
		m.info.State.String(),
		m.info.Counts.Tools,
		m.info.Counts.Prompts,
		m.info.Counts.Resources)

	return renderItem(itemStyles, m.info.Name, info, m.focused, width, m.cache, &m.m)
}

// MCPServers is a dialog that lists MCP servers and provides per-server actions.
type MCPServers struct {
	com    *common.Common
	help   help.Model
	list   *list.FilterableList
	input  textinput.Model
	ws     workspace.Workspace
	keyMap struct {
		Reconnect,
		RefreshTools,
		RefreshPrompts,
		RefreshResources,
		Next,
		Previous,
		UpDown,
		Close key.Binding
	}
}

var _ Dialog = (*MCPServers)(nil)

// NewMCPServers creates a new MCP servers dialog.
func NewMCPServers(com *common.Common, ws workspace.Workspace) *MCPServers {
	d := &MCPServers{
		com: com,
		ws:  ws,
	}

	help := help.New()
	help.Styles = com.Styles.DialogHelpStyles()
	d.help = help

	d.list = list.NewFilterableList(d.serverItems()...)
	d.list.Focus()
	d.list.SetSelected(0)

	d.input = textinput.New()
	d.input.SetVirtualCursor(false)
	d.input.Placeholder = "Type to filter"
	d.input.SetStyles(com.Styles.TextInput)
	d.input.Focus()

	d.keyMap.Reconnect = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "reconnect"),
	)
	d.keyMap.RefreshTools = key.NewBinding(
		key.WithKeys("ctrl+t"),
		key.WithHelp("ctrl+t", "refresh tools"),
	)
	d.keyMap.RefreshPrompts = key.NewBinding(
		key.WithKeys("ctrl+p"),
		key.WithHelp("ctrl+p", "refresh prompts"),
	)
	d.keyMap.RefreshResources = key.NewBinding(
		key.WithKeys("ctrl+r"),
		key.WithHelp("ctrl+r", "refresh resources"),
	)
	d.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "choose"),
	)
	d.keyMap.Next = key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↓", "next"),
	)
	d.keyMap.Previous = key.NewBinding(
		key.WithKeys("up"),
		key.WithHelp("↑", "previous"),
	)
	closeKey := CloseKey
	closeKey.SetHelp("esc", "close")
	d.keyMap.Close = closeKey

	return d
}

// serverItems builds the list items from current MCP server states, sorted by
// name so the list order is stable across dialog opens (map iteration order is
// otherwise random).
func (d *MCPServers) serverItems() []list.FilterableItem {
	states := d.ws.MCPGetStates()
	names := make([]string, 0, len(states))
	for name := range states {
		names = append(names, name)
	}
	sort.Strings(names)
	items := make([]list.FilterableItem, 0, len(states))
	for _, name := range names {
		info := states[name]
		info.Name = name
		items = append(items, NewMCPServerItem(d.com.Styles, info))
	}
	return items
}

// ID implements Dialog.
func (d *MCPServers) ID() string { return MCPServersID }

// HandleMsg implements Dialog.
func (d *MCPServers) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, d.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, d.keyMap.Previous):
			d.list.Focus()
			if d.list.IsSelectedFirst() {
				d.list.SelectLast()
			} else {
				d.list.SelectPrev()
			}
			d.list.ScrollToSelected()
		case key.Matches(msg, d.keyMap.Next):
			d.list.Focus()
			if d.list.IsSelectedLast() {
				d.list.SelectFirst()
			} else {
				d.list.SelectNext()
			}
			d.list.ScrollToSelected()
		case key.Matches(msg, d.keyMap.RefreshTools):
			if item := d.selectedServer(); item != nil {
				return ActionMCPRefreshTools{ServerName: item.info.Name}
			}
		case key.Matches(msg, d.keyMap.RefreshPrompts):
			if item := d.selectedServer(); item != nil {
				return ActionMCPRefreshPrompts{ServerName: item.info.Name}
			}
		case key.Matches(msg, d.keyMap.RefreshResources):
			if item := d.selectedServer(); item != nil {
				return ActionMCPRefreshResources{ServerName: item.info.Name}
			}
		case key.Matches(msg, d.keyMap.Reconnect):
			if item := d.selectedServer(); item != nil {
				return ActionMCPReconnect{ServerName: item.info.Name}
			}
		default:
			var cmd tea.Cmd
			d.input, cmd = d.input.Update(msg)
			value := d.input.Value()
			d.list.SetFilter(value)
			d.list.ScrollToTop()
			d.list.SetSelected(0)
			return ActionCmd{cmd}
		}
	}
	return nil
}

// selectedServer returns the currently selected MCPServerItem, or nil.
func (d *MCPServers) selectedServer() *MCPServerItem {
	item := d.list.SelectedItem()
	if item == nil {
		return nil
	}
	if si, ok := item.(*MCPServerItem); ok {
		return si
	}
	return nil
}

// Cursor returns the cursor position relative to the dialog.
func (d *MCPServers) Cursor() *tea.Cursor {
	return InputCursor(d.com.Styles, d.input.Cursor())
}

// Draw implements Dialog.
func (d *MCPServers) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := d.com.Styles
	width := max(0, min(defaultDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(defaultDialogHeight, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.InputPrompt.GetVerticalFrameSize() + inputContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()

	d.input.SetWidth(max(0, innerWidth-t.Dialog.InputPrompt.GetHorizontalFrameSize()-1))

	listHeight := height - heightOffset
	listWidth := max(0, innerWidth-3)
	d.list.SetSize(listWidth, listHeight)
	d.help.SetWidth(innerWidth)

	rc := NewRenderContext(t, width)
	rc.Title = "MCP Servers"
	inputView := t.Dialog.InputPrompt.Render(d.input.View())
	rc.AddPart(inputView)
	listView := t.Dialog.List.Height(d.list.Height()).Render(d.list.Render())
	scrollbar := common.Scrollbar(t, listHeight, d.list.TotalHeight(), listHeight, d.list.Offset())
	if scrollbar != "" {
		listView = lipgloss.JoinHorizontal(lipgloss.Top, listView, scrollbar)
	}
	rc.AddPart(listView)
	rc.Help = d.help.View(d)

	view := rc.Render()
	cur := d.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

// ShortHelp implements help.KeyMap.
func (d *MCPServers) ShortHelp() []key.Binding {
	return []key.Binding{
		d.keyMap.UpDown,
		d.keyMap.Reconnect,
		d.keyMap.RefreshTools,
		d.keyMap.Close,
	}
}

// FullHelp implements help.KeyMap.
func (d *MCPServers) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{d.keyMap.Reconnect, d.keyMap.RefreshTools, d.keyMap.RefreshPrompts, d.keyMap.RefreshResources},
		{d.keyMap.UpDown, d.keyMap.Close},
	}
}
