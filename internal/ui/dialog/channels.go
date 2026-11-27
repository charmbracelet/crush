package dialog

import (
	"fmt"
	"slices"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/sahilm/fuzzy"
)

// ChannelsID is the identifier for the channels dialog.
const ChannelsID = "channels"

// ChannelItem wraps a channel server's ClientInfo as a filterable list item.
type ChannelItem struct {
	*list.Versioned
	info    mcp.ClientInfo
	t       *styles.Styles
	m       fuzzy.Match
	cache   map[int]string
	focused bool
}

var _ ListItem = &ChannelItem{Versioned: list.NewVersioned()}

// NewChannelItem creates a new ChannelItem.
func NewChannelItem(t *styles.Styles, info mcp.ClientInfo) *ChannelItem {
	return &ChannelItem{
		Versioned: list.NewVersioned(),
		t:         t,
		info:      info,
	}
}

// Finished implements list.Item.
func (c *ChannelItem) Finished() bool { return true }

// Filter implements ListItem.
func (c *ChannelItem) Filter() string {
	return c.info.Name
}

// ID implements ListItem.
func (c *ChannelItem) ID() string {
	return c.info.Name
}

// SetFocused implements ListItem.
func (c *ChannelItem) SetFocused(focused bool) {
	if c.focused == focused {
		return
	}
	c.cache = nil
	c.focused = focused
	if c.Versioned != nil {
		c.Bump()
	}
}

// SetMatch implements ListItem.
func (c *ChannelItem) SetMatch(match fuzzy.Match) {
	if sameFuzzyMatch(c.m, match) {
		return
	}
	c.cache = nil
	c.m = match
	if c.Versioned != nil {
		c.Bump()
	}
}

// Render implements ListItem.
func (c *ChannelItem) Render(width int) string {
	itemStyles := ListItemStyles{
		ItemBlurred:     c.t.Dialog.NormalItem,
		ItemFocused:     c.t.Dialog.SelectedItem,
		InfoTextBlurred: c.t.Dialog.ListItem.InfoBlurred,
		InfoTextFocused: c.t.Dialog.ListItem.InfoFocused,
	}

	return renderItem(itemStyles, c.info.Name, c.infoText(), c.focused, width, c.cache, &c.m)
}

// infoText is the status shown beside the server name: its connection state,
// or why a connected server is not acting as a channel, plus its tool count
// while connected.
func (c *ChannelItem) infoText() string {
	status := c.info.State.String()
	switch {
	case c.info.Channel:
	case !c.info.ChannelOptIn:
		// Declares claude/channel but was never opted in.
		status = "not enabled"
	case c.info.State == mcp.StateConnected:
		// Opted in, but the server never declared claude/channel.
		status = "no channel capability"
	}
	if c.info.State != mcp.StateConnected {
		return status
	}
	tools := "tools"
	if c.info.Counts.Tools == 1 {
		tools = "tool"
	}
	return fmt.Sprintf("%s  %d %s", status, c.info.Counts.Tools, tools)
}

// Channels is a dialog that lists MCP servers that are, or could be,
// channels: every server opted in via --channels or channel_enabled, in any
// connection state, plus channel-capable servers that were never opted in.
type Channels struct {
	com    *common.Common
	help   help.Model
	list   *list.FilterableList
	input  textinput.Model
	keyMap struct {
		Next,
		Previous,
		UpDown,
		Close key.Binding
	}
}

var _ Dialog = (*Channels)(nil)

// NewChannels creates a new channels dialog from the MCP server states the
// UI already holds. It takes them rather than fetching them because it is
// called from Update, where MCPGetStates would be a blocking HTTP round-trip
// in client/server mode; later changes arrive through SetStates.
func NewChannels(com *common.Common, states map[string]mcp.ClientInfo) *Channels {
	d := &Channels{
		com: com,
	}

	help := help.New()
	help.Styles = com.Styles.DialogHelpStyles()
	d.help = help

	d.list = list.NewFilterableList(channelItems(com.Styles, states)...)
	d.list.Focus()
	d.list.SetSelected(0)

	d.input = textinput.New()
	d.input.SetVirtualCursor(false)
	d.input.Placeholder = "Type to filter"
	d.input.SetStyles(com.Styles.TextInput)
	d.input.Focus()

	// Channels have no per-item action, so moving through the list is
	// browsing, not choosing.
	d.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "browse"),
	)
	d.keyMap.Next = key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↓", "next"),
	)
	d.keyMap.Previous = key.NewBinding(
		key.WithKeys("up"),
		key.WithHelp("↑", "previous"),
	)
	d.keyMap.Close = CloseKey

	return d
}

// channelItems builds the list items from MCP server states, sorted by name.
// It keeps active channels, servers opted in as channels whatever their
// state (so a crashed or starting channel stays visible), and servers that
// declare the channel capability but were never opted in.
func channelItems(sty *styles.Styles, states map[string]mcp.ClientInfo) []list.FilterableItem {
	names := make([]string, 0, len(states))
	for name, info := range states {
		if info.Channel || info.ChannelOptIn || info.ChannelCapable {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	items := make([]list.FilterableItem, 0, len(names))
	for _, name := range names {
		info := states[name]
		info.Name = name
		items = append(items, NewChannelItem(sty, info))
	}
	return items
}

// SetStates refreshes the list from new MCP server states, so a channel that
// connects or drops while the dialog is open is reflected. The current
// filter and, when it is still listed, the selected server are kept.
func (d *Channels) SetStates(states map[string]mcp.ClientInfo) {
	selectedID := ""
	if sel := d.selectedChannel(); sel != nil {
		selectedID = sel.ID()
	}
	d.list.SetItems(channelItems(d.com.Styles, states)...)
	d.list.SetFilter(d.input.Value())
	d.list.SetSelected(0)
	for i, item := range d.list.FilteredItems() {
		if ci, ok := item.(*ChannelItem); ok && ci.ID() == selectedID {
			d.list.SetSelected(i)
			d.list.ScrollToSelected()
			break
		}
	}
}

// ID implements Dialog.
func (d *Channels) ID() string { return ChannelsID }

// HandleMsg implements Dialog.
func (d *Channels) HandleMsg(msg tea.Msg) Action {
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
		case msg.Code == tea.KeyEnter:
			// Channels have no per-item action yet; swallow Enter so it does
			// not fall through into the filter input.
			return nil
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

// selectedChannel returns the currently selected ChannelItem, or nil.
func (d *Channels) selectedChannel() *ChannelItem {
	item := d.list.SelectedItem()
	if item == nil {
		return nil
	}
	if ci, ok := item.(*ChannelItem); ok {
		return ci
	}
	return nil
}

// Cursor returns the cursor position relative to the dialog.
func (d *Channels) Cursor() *tea.Cursor {
	return InputCursor(d.com.Styles, d.input.Cursor())
}

// Draw implements Dialog.
func (d *Channels) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := d.com.Styles
	width := max(0, min(defaultDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(defaultDialogHeight, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	d.input.SetWidth(dialogInputTextWidth(t, d.input, innerWidth))
	listHeight, listTotalHeight, _ := sizeDialogList(t, d.list, innerWidth, height)

	rc := NewRenderContext(t, width)
	rc.Title = "Channels"
	inputView := t.Dialog.InputPrompt.Render(d.input.View())
	rc.AddPart(inputView)
	// With no room for list rows, skip the body: an empty list still
	// renders a line plus its margin, pushing the dialog past the area.
	if listHeight > 0 {
		msg := emptyNoMatches
		if len(d.list.Items()) == 0 {
			msg = "No channels."
		}
		listBody := renderDialogListBody(t, d.list.Render(), len(d.list.FilteredItems()) == 0, msg, innerWidth)
		listView := t.Dialog.List.Height(d.list.Height()).Render(listBody)
		rc.AddPart(joinScrollbar(t, listView, listHeight, listTotalHeight, listHeight, d.list.Offset()))
	}
	rc.Help = renderDialogHelp(t, &d.help, d, innerWidth)

	view := rc.Render()
	cur := d.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

// ShortHelp implements help.KeyMap.
func (d *Channels) ShortHelp() []key.Binding {
	return []key.Binding{
		d.keyMap.UpDown,
		d.keyMap.Close,
	}
}

// FullHelp implements help.KeyMap.
func (d *Channels) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{d.keyMap.UpDown, d.keyMap.Close},
	}
}
