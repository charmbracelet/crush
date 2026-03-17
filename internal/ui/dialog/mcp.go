package dialog

import (
	"maps"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	agentmcp "github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/util"
	uv "github.com/charmbracelet/ultraviolet"
)

// MCPID is the identifier for the MCP management dialog.
const MCPID = "mcp"

// MCP renders configured MCP servers and their current runtime state.
type MCP struct {
	com    *common.Common
	help   help.Model
	list   *list.FilterableList
	input  textinput.Model
	states map[string]agentmcp.ClientInfo

	keyMap struct {
		Select       key.Binding
		Authenticate key.Binding
		Reconnect    key.Binding
		Next         key.Binding
		Previous     key.Binding
		UpDown       key.Binding
		Close        key.Binding
	}
}

var _ Dialog = (*MCP)(nil)

// NewMCP creates a new MCP management dialog.
func NewMCP(com *common.Common, states map[string]agentmcp.ClientInfo) (*MCP, error) {
	d := &MCP{
		com:    com,
		states: maps.Clone(states),
	}

	helpView := help.New()
	helpView.Styles = com.Styles.DialogHelpStyles()
	d.help = helpView

	d.list = list.NewFilterableList()
	d.list.Focus()
	d.list.SetSelected(0)

	d.input = textinput.New()
	d.input.SetVirtualCursor(false)
	d.input.Placeholder = "Type to filter"
	d.input.SetStyles(com.Styles.TextInput)
	d.input.Focus()

	d.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "open action"),
	)
	d.keyMap.Authenticate = key.NewBinding(
		key.WithKeys("ctrl+a"),
		key.WithHelp("ctrl+a", "authenticate"),
	)
	d.keyMap.Reconnect = key.NewBinding(
		key.WithKeys("ctrl+r"),
		key.WithHelp("ctrl+r", "reconnect"),
	)
	d.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓", "next item"),
	)
	d.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", "previous item"),
	)
	d.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "choose"),
	)
	d.keyMap.Close = CloseKey

	d.refreshItems("")
	return d, nil
}

// ID implements Dialog.
func (d *MCP) ID() string {
	return MCPID
}

// SetStates updates the MCP states shown by the dialog.
func (d *MCP) SetStates(states map[string]agentmcp.ClientInfo) {
	d.states = maps.Clone(states)
	selectedID := ""
	if selected := d.list.SelectedItem(); selected != nil {
		if item, ok := selected.(*MCPItem); ok {
			selectedID = item.ID()
		}
	}
	d.refreshItems(selectedID)
}

// HandleMsg implements Dialog.
func (d *MCP) HandleMsg(msg tea.Msg) Action {
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
		case key.Matches(msg, d.keyMap.Authenticate):
			item := d.selectedItem()
			if item == nil {
				break
			}
			if !item.CanAuthenticate() {
				return ActionCmd{Cmd: util.ReportWarn("This MCP server cannot authenticate interactively.")}
			}
			return ActionAuthenticateMCP{Name: item.name}
		case key.Matches(msg, d.keyMap.Reconnect):
			item := d.selectedItem()
			if item == nil {
				break
			}
			if !item.CanReconnect() {
				return ActionCmd{Cmd: util.ReportWarn("This MCP server is disabled.")}
			}
			return ActionReconnectMCP{Name: item.name}
		case key.Matches(msg, d.keyMap.Select):
			item := d.selectedItem()
			if item == nil {
				break
			}
			action := item.DefaultAction()
			if action == nil {
				return ActionCmd{Cmd: util.ReportWarn("No action available for this MCP server.")}
			}
			return action
		default:
			var cmd tea.Cmd
			d.input, cmd = d.input.Update(msg)
			d.refreshItems("")
			return ActionCmd{Cmd: cmd}
		}
	}
	return nil
}

// Cursor returns the cursor position relative to the dialog.
func (d *MCP) Cursor() *tea.Cursor {
	return InputCursor(d.com.Styles, d.input.Cursor())
}

// Draw implements Dialog.
func (d *MCP) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := d.com.Styles
	width := max(0, min(defaultDialogMaxWidth+20, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(defaultDialogHeight+6, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.InputPrompt.GetVerticalFrameSize() + inputContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()

	d.input.SetWidth(max(0, innerWidth-t.Dialog.InputPrompt.GetHorizontalFrameSize()-1))
	d.list.SetSize(innerWidth, height-heightOffset)
	d.help.SetWidth(innerWidth)

	rc := NewRenderContext(t, width)
	rc.Title = "MCP Servers"
	rc.TitleInfo = t.HalfMuted.Render("inspect status and trigger auth/reconnect")
	rc.AddPart(t.Dialog.InputPrompt.Render(d.input.View()))
	rc.AddPart(t.Dialog.List.Height(d.list.Height()).Render(d.list.Render()))
	rc.Help = d.help.View(d)

	view := rc.Render()
	cur := d.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

// ShortHelp implements help.KeyMap.
func (d *MCP) ShortHelp() []key.Binding {
	return []key.Binding{
		d.keyMap.UpDown,
		d.keyMap.Select,
		d.keyMap.Authenticate,
		d.keyMap.Reconnect,
		d.keyMap.Close,
	}
}

// FullHelp implements help.KeyMap.
func (d *MCP) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{d.keyMap.Select, d.keyMap.Authenticate, d.keyMap.Reconnect},
		{d.keyMap.Next, d.keyMap.Previous, d.keyMap.Close},
	}
}

func (d *MCP) selectedItem() *MCPItem {
	selected := d.list.SelectedItem()
	if selected == nil {
		return nil
	}
	item, ok := selected.(*MCPItem)
	if !ok {
		return nil
	}
	return item
}

func (d *MCP) refreshItems(selectedID string) {
	items := mcpDialogItems(d.com.Styles, d.com.Config().MCP, d.states)
	d.list.SetItems(items...)
	d.list.SetFilter(d.input.Value())
	if len(d.list.FilteredItems()) == 0 {
		d.list.SetSelected(0)
		d.list.ScrollToTop()
		return
	}
	if selectedID != "" {
		for idx, item := range d.list.FilteredItems() {
			if listItem, ok := item.(*MCPItem); ok && listItem.ID() == selectedID {
				d.list.SetSelected(idx)
				d.list.ScrollToSelected()
				return
			}
		}
	}
	d.list.SetSelected(0)
	d.list.ScrollToTop()
}
