package model

import (
	"errors"
	"image"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/workspace"
	uv "github.com/charmbracelet/ultraviolet"
)

type channelsTestWorkspace struct {
	workspace.Workspace
	cfg *config.Config
}

func (w *channelsTestWorkspace) Config() *config.Config { return w.cfg }

func (w *channelsTestWorkspace) MCPPendingAuth() []mcp.PendingAuthServer { return nil }

// newChannelsTestUI builds a UI whose Config lists the given MCP names and
// whose mcpStates map holds the given per-server ClientInfo.
func newChannelsTestUI(t *testing.T, mcpNames []string, states map[string]mcp.ClientInfo) *UI {
	t.Helper()
	mcps := config.MCPs{}
	for _, n := range mcpNames {
		mcps[n] = config.MCPConfig{}
	}
	com := &common.Common{
		Workspace: &channelsTestWorkspace{cfg: &config.Config{MCP: mcps}},
		Styles:    common.DefaultCommon(nil).Styles,
	}
	return &UI{com: com, mcpStates: states}
}

// TestChannelStatusItems_FiltersSortsAndMapsState covers the core logic:
// servers opted in as channels are listed whatever their connection state,
// in name order, and each state maps to the expected status text (including
// the error message). Servers that are merely channel-capable, or plain MCP
// servers, are not listed.
func TestChannelStatusItems_FiltersSortsAndMapsState(t *testing.T) {
	t.Parallel()

	states := map[string]mcp.ClientInfo{
		"zeta-chan": {Name: "zeta-chan", State: mcp.StateConnected, Channel: true, ChannelOptIn: true},
		// Opted in but crashed: Channel is false, yet it must stay listed.
		"alpha-chan": {Name: "alpha-chan", State: mcp.StateError, ChannelOptIn: true, Error: errors.New("boom")},
		// Opted in and connected, but the server never declared claude/channel.
		"mid-chan":  {Name: "mid-chan", State: mcp.StateConnected, ChannelOptIn: true},
		"capable":   {Name: "capable", State: mcp.StateConnected, ChannelCapable: true}, // not opted in
		"plain-mcp": {Name: "plain-mcp", State: mcp.StateConnected},                     // not a channel
		// "orphan" has an MCP config but no entry in mcpStates → excluded.
	}
	m := newChannelsTestUI(t, []string{"zeta-chan", "alpha-chan", "mid-chan", "capable", "plain-mcp", "orphan"}, states)

	items := m.channelStatusItems()

	require.Len(t, items, 3, "only opted-in channels with a known state are listed")
	require.Equal(t, "alpha-chan", items[0].name)
	require.Equal(t, "mid-chan", items[1].name)
	require.Equal(t, "zeta-chan", items[2].name)

	require.Contains(t, ansi.Strip(items[0].description), "error: boom")
	require.Contains(t, ansi.Strip(items[1].description), "no channel capability")
	require.Contains(t, ansi.Strip(items[2].description), "connected")
}

// TestChannelStatusItems_StateVariants exercises the remaining state → text
// mappings for opted-in channels (starting, needs auth, a bare error, and an
// unknown state → offline).
func TestChannelStatusItems_StateVariants(t *testing.T) {
	t.Parallel()

	states := map[string]mcp.ClientInfo{
		"starting":  {Name: "starting", State: mcp.StateStarting, ChannelOptIn: true},
		"auth":      {Name: "auth", State: mcp.StateNeedsAuth, ChannelOptIn: true},
		"errorless": {Name: "errorless", State: mcp.StateError, ChannelOptIn: true}, // error state, nil Error
		"unknown":   {Name: "unknown", State: mcp.State(99), ChannelOptIn: true},    // out-of-range → offline
	}
	m := newChannelsTestUI(t, []string{"starting", "auth", "errorless", "unknown"}, states)

	got := map[string]string{}
	for _, it := range m.channelStatusItems() {
		got[it.name] = ansi.Strip(it.description)
	}
	require.Contains(t, got["starting"], "starting")
	require.Contains(t, got["auth"], "needs authentication")
	require.Equal(t, "error", got["errorless"], "error state with no Error shows bare 'error'")
	require.Contains(t, got["unknown"], "offline", "unknown state falls back to offline")
}

// TestChannelsInfo_EmptyShowsNone verifies an empty list renders the section
// title plus "None" rather than a blank section.
func TestChannelsInfo_EmptyShowsNone(t *testing.T) {
	t.Parallel()

	m := newChannelsTestUI(t, nil, nil)

	out := ansi.Strip(m.channelsInfo(nil, 40, 10, false))
	require.Contains(t, out, "Channels")
	require.Contains(t, out, "None")
}

// TestChannelList_Truncation covers the "…and N more" overflow behavior and the
// maxItems<=0 guard.
func TestChannelList_Truncation(t *testing.T) {
	t.Parallel()

	styles := common.DefaultCommon(nil).Styles
	items := []channelStatusItem{
		{name: "a", title: "a"},
		{name: "b", title: "b"},
		{name: "c", title: "c"},
	}

	// maxItems=2 with 3 items → one item shown plus a "…and 2 more" line.
	out := ansi.Strip(channelList(styles, items, 80, 2))
	require.Contains(t, out, "and 2 more")

	// Non-positive budget renders nothing.
	require.Empty(t, channelList(styles, items, 80, 0))
}

// TestMCPStateChangeRefreshesOpenChannelsDialog drives a real state-change
// message through Update and checks the open Channels dialog picks it up,
// covering the wiring rather than just the dialog's own SetStates.
func TestMCPStateChangeRefreshesOpenChannelsDialog(t *testing.T) {
	t.Parallel()

	m := newChannelsTestUI(t, []string{"signal"}, nil)
	m.dialog = dialog.NewOverlay()
	m.mcpStates = map[string]mcp.ClientInfo{
		"signal": {Name: "signal", State: mcp.StateStarting, ChannelOptIn: true},
	}
	d := dialog.NewChannels(m.com, m.mcpStates)
	m.dialog.OpenDialog(d)

	m.Update(mcpStateChangedMsg{states: map[string]mcp.ClientInfo{
		"signal":  {Name: "signal", State: mcp.StateConnected, Channel: true, ChannelOptIn: true},
		"webhook": {Name: "webhook", State: mcp.StateConnected, ChannelCapable: true},
	}})

	scr := uv.NewScreenBuffer(100, 30)
	d.Draw(scr, image.Rect(0, 0, 100, 30))
	out := ansi.Strip(scr.String())
	require.Contains(t, out, "webhook", "a server that appeared after opening should be listed")
	require.Contains(t, out, "connected", "signal's refreshed state should be shown")
}
