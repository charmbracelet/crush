package mcp

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// swapInitGate replaces the package-global initDone channel that WaitForInit
// waits on with a fresh, open one for the duration of the test, restoring the
// original in cleanup. This lets each test drive WaitForInit deterministically
// (by closing the returned channel to signal "init complete") instead of
// branching on whether an earlier test already closed the process-wide
// one-shot. The tests that use it are not parallel and nothing else touches the
// gate during a unit-test run, so the swap is race-free.
func swapInitGate(t *testing.T) chan struct{} {
	t.Helper()
	orig := initDone
	initDone = make(chan struct{})
	t.Cleanup(func() { initDone = orig })
	return initDone
}

// TestWaitForInit_BlocksUntilInitCompletes pins the contract the coordinator
// relies on: WaitForInit blocks while MCP initialization is still in flight and
// returns once it completes. The coordinator calls it before reading the tool
// registry so slow-to-start servers (e.g. stdio Python via uv) have registered
// their tools first.
func TestWaitForInit_BlocksUntilInitCompletes(t *testing.T) {
	gate := swapInitGate(t)

	// Init not done yet: WaitForInit must block until the context expires.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, WaitForInit(ctx), context.DeadlineExceeded,
		"WaitForInit must block while initialization is in flight")

	// Once initialization completes (the gate closes), WaitForInit returns nil.
	close(gate)
	require.NoError(t, WaitForInit(context.Background()),
		"WaitForInit must return once initialization has completed")
}

// TestWaitForInit_ToolsVisibleAfterInit is the regression test for the bug the
// coordinator fix addresses: buildTools read allTools concurrently with MCP
// initialization, so a slow server's tools were silently missing from the LLM's
// palette even though crush_info later reported the server as connected. Gating
// on WaitForInit fixes it — any tool registered before initialization completes
// must be visible once WaitForInit returns.
func TestWaitForInit_ToolsVisibleAfterInit(t *testing.T) {
	const name = "test-waitforinit-tools"
	t.Cleanup(func() {
		if s, ok := sessions.Take(name); ok {
			_ = s.Close()
		}
		allTools.Del(name)
		states.Del(name)
	})

	sess, _ := liveSession(t, "slow_tool")
	gate := swapInitGate(t)

	// A slow MCP server registers its tools, then initialization completes
	// (the gate closes). close(gate) happens-after the registration, and
	// WaitForInit returning happens-after observing the close, so the tools are
	// guaranteed visible once WaitForInit returns.
	go func() {
		sessions.Set(name, sess)
		allTools.Set(name, []*Tool{{Name: "slow_tool"}})
		updateState(name, StateConnected, nil, sess, Counts{Tools: 1})
		close(gate)
	}()

	require.NoError(t, WaitForInit(context.Background()))

	tools, ok := allTools.Get(name)
	require.True(t, ok, "a slow server's tools must be visible after WaitForInit returns")
	require.Len(t, tools, 1)
	require.Equal(t, "slow_tool", tools[0].Name)
}
