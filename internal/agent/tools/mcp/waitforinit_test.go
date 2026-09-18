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

	initMu.Lock()
	origStarted := initStarted
	origArmedAt := initArmedAt
	initStarted = true
	initArmedAt = time.Now()
	initMu.Unlock()

	t.Cleanup(func() {
		initDone = orig
		initMu.Lock()
		initStarted = origStarted
		initArmedAt = origArmedAt
		initMu.Unlock()
	})
	return initDone
}

// TestWaitForInit_BlocksUntilInitCompletes pins the contract the
// non-interactive path relies on: WaitForInit blocks while MCP initialization
// is still in flight and returns once it completes. Non-interactive runs
// (`crush run`) wait on it before reading the tool registry so slow-to-start
// servers (e.g. stdio Python via uv) have registered their tools first.
// Interactive runs deliberately do not gate on it (a slow server froze the
// TUI's first prompt); they build the tool palette from whatever is registered
// at send time and pick up late servers on later runs. See coordinator.run.
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

// TestWaitForInit_ReturnsWhenNotArmed is the regression test for callers
// outside app startup. Those paths never call mcp.Initialize (which is
// what arms the gate), so WaitForInit must return immediately instead of
// blocking on a channel that will never close. Before the fix it blocked
// until ctx was cancelled, hanging RunNonInteractive's gate forever.
func TestWaitForInit_ReturnsWhenNotArmed(t *testing.T) {
	// Ensure the gate looks unarmed regardless of test ordering.
	initMu.Lock()
	orig := initStarted
	initStarted = false
	initMu.Unlock()
	t.Cleanup(func() {
		initMu.Lock()
		initStarted = orig
		initMu.Unlock()
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, WaitForInit(ctx),
		"WaitForInit must return immediately when initialization was never armed")
}

// TestWaitForInitBudget_ProceedsWhenInitWedged is the regression test for the
// "messages go into the void" hang: an MCP server that never answers its
// handshake (e.g. a Python server that chokes on the SEP-2575 server/discover
// probe without responding) keeps Initialize — and so the init gate — open for
// its full connect timeout, up to minutes. Every turn gated on the open-ended
// WaitForInit, so typing a message produced nothing: no persisted user
// message, no spinner. The bounded wait must give up after its budget and let
// the turn proceed without the wedged server.
func TestWaitForInitBudget_ProceedsWhenInitWedged(t *testing.T) {
	swapInitGate(t) // armed, never closed: initialization is wedged

	start := time.Now()
	require.NoError(t, WaitForInitBudget(context.Background(), 50*time.Millisecond),
		"a wedged MCP initialization must not fail the turn once the budget elapses")
	require.Less(t, time.Since(start), 5*time.Second,
		"WaitForInitBudget must return promptly after its budget, not block until initialization finishes")
}

// TestWaitForInitBudget_ReturnsOnceInitCompletes pins that the budget is a
// ceiling, not a delay: when initialization finishes within the budget the
// wait ends immediately, preserving the #132 guarantee that a healthy
// slow-to-start server's tools are registered before buildTools reads the
// registry.
func TestWaitForInitBudget_ReturnsOnceInitCompletes(t *testing.T) {
	gate := swapInitGate(t)

	go func() {
		time.Sleep(20 * time.Millisecond)
		close(gate)
	}()

	start := time.Now()
	require.NoError(t, WaitForInitBudget(context.Background(), 30*time.Second))
	require.Less(t, time.Since(start), 5*time.Second,
		"WaitForInitBudget must return as soon as initialization completes, not sit out its budget")
}

// TestWaitForInitBudget_CallerCancellationStillAborts pins that the budget
// only absorbs its own deadline: the caller's context being cancelled (the
// user hit esc, the request ended) still aborts the turn with an error rather
// than being mistaken for an elapsed budget and silently proceeding.
func TestWaitForInitBudget_CallerCancellationStillAborts(t *testing.T) {
	swapInitGate(t) // armed, never closed

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	require.ErrorIs(t, WaitForInitBudget(ctx, 30*time.Second), context.Canceled,
		"caller cancellation must surface as an error, not be swallowed like an elapsed budget")
}

// TestWaitForInitBudget_DeadlineIsAbsolute pins that the budget anchors at
// ArmInit, not at each call: a turn's path holds several sequential waiters
// (readyWg's tool build, then the turn's own gate), and per-call budgets would
// stack into multiples of the budget while a server is wedged. A call arriving
// after the armed-at deadline has already passed must return at once.
func TestWaitForInitBudget_DeadlineIsAbsolute(t *testing.T) {
	swapInitGate(t) // armed, never closed

	// Rewind the arming time so the 30s budget is already spent.
	initMu.Lock()
	initArmedAt = time.Now().Add(-time.Minute)
	initMu.Unlock()

	start := time.Now()
	require.NoError(t, WaitForInitBudget(context.Background(), 30*time.Second),
		"a call after the armed-at deadline must proceed without waiting")
	require.Less(t, time.Since(start), 5*time.Second,
		"the budget must not restart per call once the armed-at deadline has passed")
}

// TestWaitForInitBudget_ReturnsWhenNotArmed mirrors WaitForInit's contract for
// coordinators built outside app startup: nothing armed means nothing to wait
// for.
func TestWaitForInitBudget_ReturnsWhenNotArmed(t *testing.T) {
	initMu.Lock()
	orig := initStarted
	initStarted = false
	initMu.Unlock()
	t.Cleanup(func() {
		initMu.Lock()
		initStarted = orig
		initMu.Unlock()
	})

	require.NoError(t, WaitForInitBudget(context.Background(), 30*time.Second),
		"WaitForInitBudget must return immediately when initialization was never armed")
}

// TestWaitForInit_ToolsVisibleAfterInit pins the visibility guarantee
// WaitForInit gives the non-interactive path: any tool registered before
// initialization completes must be visible once WaitForInit returns. The
// interactive coordinator deliberately no longer relies on this (it reads the
// registry ungated and picks up late tools on subsequent runs); this test
// keeps the guarantee for non-interactive runs, which still wait.
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
