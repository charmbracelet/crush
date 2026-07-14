package dialog

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

type stubChannelsWorkspace struct {
	workspace.Workspace
	states map[string]mcp.ClientInfo
}

func (w *stubChannelsWorkspace) MCPGetStates() map[string]mcp.ClientInfo {
	return w.states
}

func newTestChannels(t *testing.T, states map[string]mcp.ClientInfo) *Channels {
	t.Helper()
	s := styles.CharmtonePantera()
	com := &common.Common{Styles: &s}
	ws := &stubChannelsWorkspace{states: states}
	return NewChannels(com, ws)
}

func TestChannels_EnterReconnectsSelectedChannel(t *testing.T) {
	t.Parallel()

	d := newTestChannels(t, map[string]mcp.ClientInfo{
		"signal": {Name: "signal", State: mcp.StateConnected, Channel: true, Counts: mcp.Counts{Tools: 3}},
	})

	action := d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})

	reconnect, ok := action.(ActionMCPReconnect)
	require.True(t, ok, "expected ActionMCPReconnect")
	require.Equal(t, "signal", reconnect.ServerName)
}

func TestChannels_EscClosesDialog(t *testing.T) {
	t.Parallel()

	d := newTestChannels(t, map[string]mcp.ClientInfo{
		"signal": {Name: "signal", State: mcp.StateConnected, Channel: true},
	})

	action := d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEscape})
	_, ok := action.(ActionClose)
	require.True(t, ok, "Esc should close the dialog")
}

func TestChannels_FilterNarrowsList(t *testing.T) {
	t.Parallel()

	d := newTestChannels(t, map[string]mcp.ClientInfo{
		"signal": {Name: "signal", State: mcp.StateConnected, Channel: true},
		"slack":  {Name: "slack", State: mcp.StateError, Channel: true},
	})

	d.HandleMsg(keyMsg('s'))
	d.HandleMsg(keyMsg('i'))
	d.HandleMsg(keyMsg('g'))

	filtered := d.list.FilteredItems()
	require.Len(t, filtered, 1)
	item := filtered[0].(*ChannelItem)
	require.Equal(t, "signal", item.info.Name)
}

func TestChannels_OnlyShowsChannelServers(t *testing.T) {
	t.Parallel()

	d := newTestChannels(t, map[string]mcp.ClientInfo{
		"signal":     {Name: "signal", State: mcp.StateConnected, Channel: true},
		"nonchannel": {Name: "nonchannel", State: mcp.StateConnected, Channel: false},
	})

	items := d.list.FilteredItems()
	require.Len(t, items, 1, "only channel-capable servers should appear")
	require.Equal(t, "signal", items[0].(*ChannelItem).info.Name)
}

func TestChannels_ChannelsSortedByName(t *testing.T) {
	t.Parallel()

	d := newTestChannels(t, map[string]mcp.ClientInfo{
		"zeta":  {Name: "zeta", State: mcp.StateConnected, Channel: true},
		"alpha": {Name: "alpha", State: mcp.StateConnected, Channel: true},
		"mid":   {Name: "mid", State: mcp.StateConnected, Channel: true},
	})

	items := d.list.FilteredItems()
	require.Len(t, items, 3)
	names := make([]string, 0, len(items))
	for _, it := range items {
		names = append(names, it.(*ChannelItem).info.Name)
	}
	require.Equal(t, []string{"alpha", "mid", "zeta"}, names,
		"channels should be listed in a stable alphabetical order")
}

func TestChannels_NavigationWraps(t *testing.T) {
	t.Parallel()

	d := newTestChannels(t, map[string]mcp.ClientInfo{
		"alpha": {Name: "alpha", State: mcp.StateConnected, Channel: true},
		"beta":  {Name: "beta", State: mcp.StateConnected, Channel: true},
	})

	first := d.selectedChannel()
	require.NotNil(t, first)

	d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	second := d.selectedChannel()
	require.NotNil(t, second)
	require.NotEqual(t, first.info.Name, second.info.Name, "Down should move to a different channel")

	d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	wrapped := d.selectedChannel()
	require.NotNil(t, wrapped)
	require.Equal(t, first.info.Name, wrapped.info.Name, "Down at end should wrap to first")

	d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyUp})
	upWrapped := d.selectedChannel()
	require.NotNil(t, upWrapped)
	require.Equal(t, second.info.Name, upWrapped.info.Name, "Up at start should wrap to last")
}

func TestChannels_EmptyStatesNoPanic(t *testing.T) {
	t.Parallel()

	d := newTestChannels(t, map[string]mcp.ClientInfo{})

	action := d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Nil(t, action, "no action when no channel selected")
}

func TestChannels_NoChannelsOnlyNonChannelServers(t *testing.T) {
	t.Parallel()

	d := newTestChannels(t, map[string]mcp.ClientInfo{
		"mcp1": {Name: "mcp1", State: mcp.StateConnected, Channel: false},
		"mcp2": {Name: "mcp2", State: mcp.StateConnected, Channel: false},
	})

	items := d.list.FilteredItems()
	require.Empty(t, items, "no channels should appear when all servers are non-channel")
}

func TestChannelItem_RenderDoesNotPanic(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	info := mcp.ClientInfo{
		Name:    "test",
		State:   mcp.StateConnected,
		Channel: true,
		Counts:  mcp.Counts{Tools: 5},
	}
	item := NewChannelItem(&s, info)
	rendered := item.Render(60)
	require.NotEmpty(t, rendered)
}

func TestChannelItem_FilterMatchesName(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	item := NewChannelItem(&s, mcp.ClientInfo{Name: "signal-channel"})
	require.Equal(t, "signal-channel", item.Filter())
}
