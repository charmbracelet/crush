package model

import (
	"errors"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

// TestMCPEventRefreshIsOffThreadAndDeduped pins the MCP side of the
// no-IO-in-Update invariant: MCPGetStates is a synchronous HTTP round-trip
// in client/server mode, so a state_changed event must schedule one
// off-thread fetch, dedup while one is in flight, and re-dispatch a queued
// refresh when the in-flight fetch lands.
func TestMCPEventRefreshIsOffThreadAndDeduped(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{
		ready: true,
		mcpStates: map[string]mcp.ClientInfo{
			"ctx": {Name: "ctx", State: mcp.StateConnected, Counts: mcp.Counts{Tools: 4, Prompts: 2}},
		},
	}
	m := newBusyUI(ws)
	warmCaches(m, false)
	ws.resetCounters()

	_, cmd := m.Update(pubsub.Event[mcp.Event]{
		Type:    pubsub.UpdatedEvent,
		Payload: mcp.Event{Type: mcp.EventStateChanged, Name: "ctx", State: mcp.StateConnected},
	})
	require.Zero(t, ws.syncProbes(), "the MCP event handler must not probe synchronously")
	require.True(t, m.mcpFetchInFlight, "an MCP state change must schedule an off-thread refresh")

	// A second event while the fetch is in flight queues a re-fetch instead
	// of stacking another dispatch.
	m.Update(pubsub.Event[mcp.Event]{
		Type:    pubsub.UpdatedEvent,
		Payload: mcp.Event{Type: mcp.EventStateChanged, Name: "ctx", State: mcp.StateConnected},
	})
	require.Zero(t, ws.syncProbes())
	require.True(t, m.mcpRefreshQueued, "an event during an in-flight fetch must queue a re-fetch")

	runCmds(m, cmd)
	require.False(t, m.mcpFetchInFlight)
	require.False(t, m.mcpRefreshQueued, "the queued flag must clear once the re-dispatched fetch lands")
	require.Equal(t, 4, m.mcpStates["ctx"].Counts.Tools, "fetched states must land in the cache")
	require.Equal(t, 2, m.mcpStates["ctx"].Counts.Prompts, "fetched prompt counts must land in the cache")
	require.Equal(t, 2, ws.mcpStateCalls, "one fetch plus the queued re-fetch")
}

// TestMCPStartingRetryLoop pins the convergence fix for the stuck
// "starting..." case: a server still connecting arms a re-probe tick, so the
// sidebar settles without depending on state_changed events (which can be
// missed in client/server mode when the SSE stream attaches late, drops and
// reconnects, or the broker drops an event). Terminal states disarm the
// loop.
func TestMCPStartingRetryLoop(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{
		ready: true,
		mcpStates: map[string]mcp.ClientInfo{
			"slow": {Name: "slow", State: mcp.StateStarting},
		},
	}
	m := newBusyUI(ws)
	warmCaches(m, false)

	cmds := m.applyMCPStates(mcpStateChangedMsg{states: map[string]mcp.ClientInfo{
		"slow": {Name: "slow", State: mcp.StateStarting},
	}})
	require.Len(t, cmds, 1, "a starting server must arm the retry tick")
	require.Equal(t, mcp.StateStarting, m.mcpStates["slow"].State)

	// The server settles before the tick fires; the re-probe lands the
	// connected state and the loop stops.
	ws.mcpStates = map[string]mcp.ClientInfo{
		"slow": {Name: "slow", State: mcp.StateConnected, Counts: mcp.Counts{Tools: 2}},
	}
	_, cmd := m.Update(mcpStartingRetryMsg{})
	require.True(t, m.mcpFetchInFlight, "the retry tick must re-dispatch a state refresh")
	runCmds(m, cmd)
	require.False(t, m.mcpFetchInFlight)
	require.Equal(t, mcp.StateConnected, m.mcpStates["slow"].State,
		"the retry loop must converge on the settled state")

	cmds = m.applyMCPStates(mcpStateChangedMsg{states: map[string]mcp.ClientInfo{
		"slow": {Name: "slow", State: mcp.StateConnected, Counts: mcp.Counts{Tools: 2}},
	}})
	require.Empty(t, cmds, "terminal states must not arm the retry tick")
}

// TestMCPStatesTTLBackstop pins the backstop in the Update tail: when the
// memoized MCP states outlive their TTL — the safety valve for state_changed
// events missed in client/server mode — a refresh is re-dispatched.
func TestMCPStatesTTLBackstop(t *testing.T) {
	ws := &countingWorkspace{ready: true}
	m := newBusyUI(ws)

	// The check time starts zero, so the first Update sees the MCP cache as
	// stale and dispatches a refresh.
	_, cmd := m.Update(plainMsg{})
	require.True(t, m.mcpFetchInFlight, "a stale MCP cache must re-dispatch a refresh")
	runCmds(m, cmd)
	require.False(t, m.mcpFetchInFlight)

	// A fresh stamp keeps the backstop quiet.
	m.Update(plainMsg{})
	require.False(t, m.mcpFetchInFlight, "a fresh MCP cache must not re-dispatch")

	// Once the TTL elapses, the next Update re-dispatches.
	m.mcpCheckedAt = time.Now().Add(-2 * mcpStatesTTL)
	m.Update(plainMsg{})
	require.True(t, m.mcpFetchInFlight, "an expired MCP cache must re-dispatch a refresh")
}

// TestMCPFailedFetchKeepsLastKnownGood pins the error path: a fetch that
// fails (nil states, e.g. a transient HTTP error in client/server mode)
// keeps the last-known-good states on screen instead of blanking the
// sidebar.
func TestMCPFailedFetchKeepsLastKnownGood(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{
		ready: true,
		mcpStates: map[string]mcp.ClientInfo{
			"ctx": {Name: "ctx", State: mcp.StateConnected, Counts: mcp.Counts{Tools: 3}},
		},
	}
	m := newBusyUI(ws)
	warmCaches(m, false)

	runCmds(m, m.requestMCPRefresh())
	require.Equal(t, mcp.StateConnected, m.mcpStates["ctx"].State,
		"the seeded states must land in the cache")

	// Simulate the next fetch failing (the client workspace returns nil
	// states when the round-trip errors).
	cmds := m.applyMCPStates(mcpStateChangedMsg{states: nil})
	require.Empty(t, cmds)
	require.False(t, m.mcpFetchInFlight)
	require.Equal(t, mcp.StateConnected, m.mcpStates["ctx"].State,
		"a failed fetch must keep the last-known-good states")
}

// TestConnectionRecoveredResyncsMCPStates pins the reconnect recovery: the
// subscription loop documents that events published while the SSE stream was
// down are gone, so recovery must re-sync the memoized MCP states.
func TestConnectionRecoveredResyncsMCPStates(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true}
	m := newBusyUI(ws)
	warmCaches(m, false)
	ws.resetCounters()

	m.Update(workspace.ConnectionEvent{State: workspace.ConnectionRecovered})
	require.Zero(t, ws.syncProbes(), "the recovery handler must not probe synchronously")
	require.True(t, m.mcpFetchInFlight,
		"recovery must re-sync the MCP states: events published while the stream was down are gone")
}

// TestMCPAuthCompleteRefreshesStates pins the OAuth completion path: when
// the dialog reports the flow finished, the memoized states must refresh so
// the sidebar reflects the outcome even when the state_changed event was
// missed.
func TestMCPAuthCompleteRefreshesStates(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true}
	m := newBusyUI(ws)
	warmCaches(m, false)
	ws.resetCounters()

	for _, action := range []tea.Msg{
		dialog.ActionMCPAuthComplete{Name: "ctx"},
		dialog.ActionMCPAuthErrored{Name: "ctx", Error: errors.New("authentication timed out")},
	} {
		m.mcpFetchInFlight = false
		m.mcpCheckedAt = time.Now()
		m.Update(action)
		require.Zero(t, ws.mcpStateCalls, "the auth action handler must not probe synchronously")
		require.True(t, m.mcpFetchInFlight,
			"auth completion must refresh the MCP states (%T)", action)
	}
}

// TestMCPNeedsAuthDialogOpensOnTransitionOnly pins the dialog gating: the
// auth dialog opens when a server newly enters StateNeedsAuth (including the
// first observation), but backstop-driven refreshes with unchanged states
// must not re-open a dialog the user already dismissed.
func TestMCPNeedsAuthDialogOpensOnTransitionOnly(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{
		ready:          true,
		mcpPendingAuth: []mcp.PendingAuthServer{{Name: "ctx", URL: "https://example.com/mcp"}},
	}
	m := newBusyUI(ws)
	warmCaches(m, false)

	m.applyMCPStates(mcpStateChangedMsg{states: map[string]mcp.ClientInfo{
		"ctx": {Name: "ctx", State: mcp.StateNeedsAuth},
	}})
	require.Equal(t, 1, ws.mcpPendingCalls, "a newly needs-auth server must prompt authentication")
	require.True(t, m.dialog.ContainsDialog(dialog.MCPAuthID))

	// Unchanged needs-auth states (a backstop refresh, not a transition)
	// must not re-prompt.
	m.applyMCPStates(mcpStateChangedMsg{states: map[string]mcp.ClientInfo{
		"ctx": {Name: "ctx", State: mcp.StateNeedsAuth},
	}})
	require.Equal(t, 1, ws.mcpPendingCalls, "unchanged needs-auth states must not re-prompt")
}
