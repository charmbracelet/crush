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

type stubMCPWorkspace struct {
	workspace.Workspace
	states map[string]mcp.ClientInfo
}

func (w *stubMCPWorkspace) MCPGetStates() map[string]mcp.ClientInfo {
	return w.states
}

func newTestMCPServers(t *testing.T, states map[string]mcp.ClientInfo) *MCPServers {
	t.Helper()
	s := styles.CharmtonePantera()
	com := &common.Common{Styles: &s}
	ws := &stubMCPWorkspace{states: states}
	return NewMCPServers(com, ws)
}

func TestMCPServers_EnterReconnectsSelectedServer(t *testing.T) {
	t.Parallel()

	states := map[string]mcp.ClientInfo{
		"signal": {Name: "signal", State: mcp.StateConnected, Counts: mcp.Counts{Tools: 3, Prompts: 1, Resources: 0}},
		"cairn":  {Name: "cairn", State: mcp.StateError, Counts: mcp.Counts{}},
	}
	d := newTestMCPServers(t, states)

	// Select the first server and press Enter — should fire reconnect for that server.
	action := d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})

	reconnect, ok := action.(ActionMCPReconnect)
	require.True(t, ok, "expected ActionMCPReconnect")
	require.Contains(t, []string{"signal", "cairn"}, reconnect.ServerName)
}

func TestMCPServers_EscClosesDialog(t *testing.T) {
	t.Parallel()

	d := newTestMCPServers(t, map[string]mcp.ClientInfo{
		"signal": {Name: "signal", State: mcp.StateConnected},
	})

	action := d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEscape})
	_, ok := action.(ActionClose)
	require.True(t, ok, "Esc should close the dialog")
}

func TestMCPServers_RefreshToolsAction(t *testing.T) {
	t.Parallel()

	d := newTestMCPServers(t, map[string]mcp.ClientInfo{
		"signal": {Name: "signal", State: mcp.StateConnected},
	})

	action := d.HandleMsg(tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl})
	refresh, ok := action.(ActionMCPRefreshTools)
	require.True(t, ok, "expected ActionMCPRefreshTools")
	require.Equal(t, "signal", refresh.ServerName)
}

func TestMCPServers_RefreshPromptsAction(t *testing.T) {
	t.Parallel()

	d := newTestMCPServers(t, map[string]mcp.ClientInfo{
		"signal": {Name: "signal", State: mcp.StateConnected},
	})

	action := d.HandleMsg(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
	refresh, ok := action.(ActionMCPRefreshPrompts)
	require.True(t, ok, "expected ActionMCPRefreshPrompts")
	require.Equal(t, "signal", refresh.ServerName)
}

func TestMCPServers_RefreshResourcesAction(t *testing.T) {
	t.Parallel()

	d := newTestMCPServers(t, map[string]mcp.ClientInfo{
		"signal": {Name: "signal", State: mcp.StateConnected},
	})

	action := d.HandleMsg(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	refresh, ok := action.(ActionMCPRefreshResources)
	require.True(t, ok, "expected ActionMCPRefreshResources")
	require.Equal(t, "signal", refresh.ServerName)
}

func TestMCPServers_FilterNarrowsList(t *testing.T) {
	t.Parallel()

	d := newTestMCPServers(t, map[string]mcp.ClientInfo{
		"signal": {Name: "signal", State: mcp.StateConnected},
		"cairn":  {Name: "cairn", State: mcp.StateError},
	})

	// Type "cai" to filter to just the cairn server.
	d.HandleMsg(keyMsg('c'))
	d.HandleMsg(keyMsg('a'))
	d.HandleMsg(keyMsg('i'))

	filtered := d.list.FilteredItems()
	require.Len(t, filtered, 1)
	item := filtered[0].(*MCPServerItem)
	require.Equal(t, "cairn", item.info.Name)
}

func TestMCPServers_ServersSortedByName(t *testing.T) {
	t.Parallel()

	d := newTestMCPServers(t, map[string]mcp.ClientInfo{
		"zeta":  {Name: "zeta", State: mcp.StateConnected},
		"alpha": {Name: "alpha", State: mcp.StateConnected},
		"mid":   {Name: "mid", State: mcp.StateConnected},
	})

	items := d.list.FilteredItems()
	require.Len(t, items, 3)
	names := make([]string, 0, len(items))
	for _, it := range items {
		names = append(names, it.(*MCPServerItem).info.Name)
	}
	require.Equal(t, []string{"alpha", "mid", "zeta"}, names,
		"servers should be listed in a stable alphabetical order")
}

func TestMCPServers_NavigationWraps(t *testing.T) {
	t.Parallel()

	d := newTestMCPServers(t, map[string]mcp.ClientInfo{
		"alpha": {Name: "alpha", State: mcp.StateConnected},
		"beta":  {Name: "beta", State: mcp.StateConnected},
	})

	// Get the first selected server.
	first := d.selectedServer()
	require.NotNil(t, first)

	// Down should move to the other server.
	d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	second := d.selectedServer()
	require.NotNil(t, second)
	require.NotEqual(t, first.info.Name, second.info.Name, "Down should move to a different server")

	// Down again should wrap back to the first.
	d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	wrapped := d.selectedServer()
	require.NotNil(t, wrapped)
	require.Equal(t, first.info.Name, wrapped.info.Name, "Down at end should wrap to first")

	// Up should wrap to last.
	d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyUp})
	upWrapped := d.selectedServer()
	require.NotNil(t, upWrapped)
	require.Equal(t, second.info.Name, upWrapped.info.Name, "Up at start should wrap to last")
}

func TestMCPServers_EmptyStatesNoPanic(t *testing.T) {
	t.Parallel()

	d := newTestMCPServers(t, map[string]mcp.ClientInfo{})

	// Should not panic when there are no servers.
	action := d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Nil(t, action, "no action when no server selected")
}

func TestMCPServerItem_RenderDoesNotPanic(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	info := mcp.ClientInfo{
		Name:   "test",
		State:  mcp.StateConnected,
		Counts: mcp.Counts{Tools: 5, Prompts: 2, Resources: 1},
	}
	item := NewMCPServerItem(&s, info)
	rendered := item.Render(60)
	require.NotEmpty(t, rendered)
}

func TestMCPServerItem_FilterMatchesName(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	item := NewMCPServerItem(&s, mcp.ClientInfo{Name: "signal-server"})
	require.Equal(t, "signal-server", item.Filter())
}
