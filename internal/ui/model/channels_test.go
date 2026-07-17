package model

import (
	"errors"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/workspace"
)

type channelsTestWorkspace struct {
	workspace.Workspace
	cfg *config.Config
}

func (w *channelsTestWorkspace) Config() *config.Config { return w.cfg }

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

// TestChannelStatusItems_FiltersSortsAndMapsState covers the core logic: only
// MCP servers that are active channels (Channel==true) with a known state are
// listed, they are sorted by name, and each connection state maps to the
// expected status text (including the error message).
func TestChannelStatusItems_FiltersSortsAndMapsState(t *testing.T) {
	t.Parallel()

	states := map[string]mcp.ClientInfo{
		"zeta-chan":  {Name: "zeta-chan", State: mcp.StateConnected, Channel: true},
		"alpha-chan": {Name: "alpha-chan", State: mcp.StateError, Channel: true, Error: errors.New("boom")},
		"plain-mcp":  {Name: "plain-mcp", State: mcp.StateConnected, Channel: false}, // not a channel
		// "orphan" has an MCP config but no entry in mcpStates → excluded.
	}
	m := newChannelsTestUI(t, []string{"zeta-chan", "alpha-chan", "plain-mcp", "orphan"}, states)

	items := m.channelStatusItems()

	// Only the two Channel==true servers that have a state, sorted by name.
	require.Len(t, items, 2, "only active channels with a known state are listed")
	require.Equal(t, "alpha-chan", items[0].name)
	require.Equal(t, "zeta-chan", items[1].name)

	// Error channel surfaces its error text; connected channel shows "connected".
	require.Contains(t, ansi.Strip(items[0].description), "error: boom")
	require.Contains(t, ansi.Strip(items[1].description), "connected")
}

// TestChannelStatusItems_StateVariants exercises the remaining state → text
// mappings (starting, disabled, and an unknown state → offline).
func TestChannelStatusItems_StateVariants(t *testing.T) {
	t.Parallel()

	states := map[string]mcp.ClientInfo{
		"starting":  {Name: "starting", State: mcp.StateStarting, Channel: true},
		"disabled":  {Name: "disabled", State: mcp.StateDisabled, Channel: true},
		"errorless": {Name: "errorless", State: mcp.StateError, Channel: true}, // error state, nil Error
		"unknown":   {Name: "unknown", State: mcp.State(99), Channel: true},    // out-of-range → offline
	}
	m := newChannelsTestUI(t, []string{"starting", "disabled", "errorless", "unknown"}, states)

	got := map[string]string{}
	for _, it := range m.channelStatusItems() {
		got[it.name] = ansi.Strip(it.description)
	}
	require.Contains(t, got["starting"], "starting")
	require.Contains(t, got["disabled"], "disabled")
	require.Equal(t, "error", got["errorless"], "error state with no Error shows bare 'error'")
	require.Contains(t, got["unknown"], "offline", "unknown state falls back to offline")
}

// TestChannelsInfo_EmptyShowsNone verifies the empty state renders the section
// title plus "None" when no channels are configured.
func TestChannelsInfo_EmptyShowsNone(t *testing.T) {
	t.Parallel()

	m := newChannelsTestUI(t, []string{"plain"}, map[string]mcp.ClientInfo{
		"plain": {Name: "plain", State: mcp.StateConnected, Channel: false},
	})

	out := ansi.Strip(m.channelsInfo(40, 10, false))
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
