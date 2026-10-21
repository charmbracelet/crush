package agent

import (
	"testing"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/stretchr/testify/require"
)

// newBusyTestCoordinator builds a coordinator wired to one real agent, with
// the sub-session tracking a live sub-agent would populate.
func newBusyTestCoordinator(main SessionAgent) *coordinator {
	return &coordinator{
		mainAgent:         main,
		mainAgentName:     "coder",
		subSessionParents: csync.NewMap[string, string](),
	}
}

// A prompt is accepted before its run registers as active. Anything asking
// whether a session may be modified must be told it is busy in that window,
// or it acts on a session that is about to run.
func TestHasPendingWork_CoversAcceptedButNotYetActiveRuns(t *testing.T) {
	t.Parallel()
	sa, _ := newCancelTestAgent(t)

	require.False(t, sa.HasPendingWork("sid"), "an untouched session is idle")

	accept := sa.BeginAccepted("sid")
	require.True(t, sa.HasPendingWork("sid"),
		"a session with an accepted run is busy before the run starts")

	accept.Close()
	require.False(t, sa.HasPendingWork("sid"), "closing the last accept frees it")
}

// Sibling accepts must not free the session until the last one closes.
func TestHasPendingWork_StaysBusyWhileAnyAcceptRemains(t *testing.T) {
	t.Parallel()
	sa, _ := newCancelTestAgent(t)

	first := sa.BeginAccepted("sid")
	second := sa.BeginAccepted("sid")

	first.Close()
	require.True(t, sa.HasPendingWork("sid"), "one accept still outstanding")

	second.Close()
	require.False(t, sa.HasPendingWork("sid"))
}

// Run and Summarize ask HasActiveTurn to decide whether to queue behind an
// existing turn. A caller holds its own accept reservation while it asks,
// so counting accepts there would make a run queue behind itself.
func TestHasActiveTurn_IgnoresTheCallersOwnAccept(t *testing.T) {
	t.Parallel()
	sa, _ := newCancelTestAgent(t)

	accept := sa.BeginAccepted("sid")
	t.Cleanup(accept.Close)

	require.False(t, sa.HasActiveTurn("sid"),
		"an accept alone is not an active turn")
	require.True(t, sa.HasPendingWork("sid"),
		"but observers must still see work in flight")
}

// A sub-agent runs in its own session against its own agent instance, so
// the agent knows nothing about that session. The run holding it is the
// parent's, and callers that repair an "idle" session would otherwise
// write into a sub-agent whose tools are still running.
func TestCoordinatorHasPendingWork_ResolvesSubSessionThroughParent(t *testing.T) {
	t.Parallel()
	main, _ := newCancelTestAgent(t)
	coord := newBusyTestCoordinator(main)

	forget := coord.trackSubSession("child", "parent")
	require.False(t, coord.HasPendingWork("child"), "nothing running yet")

	accept := main.BeginAccepted("parent")
	t.Cleanup(accept.Close)

	require.True(t, coord.HasPendingWork("parent"), "the parent is busy")
	require.True(t, coord.HasPendingWork("child"),
		"a sub-session is busy while the run that owns it is")

	// Once the sub-agent finishes, its session is nobody's live work.
	forget()
	require.False(t, coord.HasPendingWork("child"))
}

// A sub-agent can dispatch its own sub-agent, so ownership is a chain
// rather than a single hop. Resolving only one level would call a
// grandchild idle while the run at the top is still going.
func TestCoordinatorHasPendingWork_ResolvesNestedSubSessions(t *testing.T) {
	t.Parallel()
	main, _ := newCancelTestAgent(t)
	coord := newBusyTestCoordinator(main)

	t.Cleanup(coord.trackSubSession("child", "root"))
	t.Cleanup(coord.trackSubSession("grandchild", "child"))

	accept := main.BeginAccepted("root")
	t.Cleanup(accept.Close)

	require.True(t, coord.HasPendingWork("grandchild"),
		"ownership is followed all the way to the run that holds it")
}

// A cycle in the ownership chain must not hang the caller. It should not
// be reachable, which is exactly why it has to fail safely if it ever is.
func TestCoordinatorHasPendingWork_TerminatesOnACycle(t *testing.T) {
	t.Parallel()
	main, _ := newCancelTestAgent(t)
	coord := newBusyTestCoordinator(main)

	t.Cleanup(coord.trackSubSession("a", "b"))
	t.Cleanup(coord.trackSubSession("b", "a"))

	require.False(t, coord.HasPendingWork("a"), "returns rather than looping")
}

// A top-level session has no owner to fall back on, and an unknown session
// must not report busy just because it is unknown.
func TestCoordinatorHasPendingWork_TopLevelAndUnknownSessions(t *testing.T) {
	t.Parallel()
	main, _ := newCancelTestAgent(t)
	coord := newBusyTestCoordinator(main)

	require.False(t, coord.HasPendingWork("solo"))
	require.False(t, coord.HasPendingWork("never-heard-of-it"))

	accept := main.BeginAccepted("solo")
	t.Cleanup(accept.Close)
	require.True(t, coord.HasPendingWork("solo"))
}

// Sub-agents under different parents must not be confused: a run in one
// parent says nothing about another parent's children.
func TestCoordinatorHasPendingWork_DoesNotLeakAcrossParents(t *testing.T) {
	t.Parallel()
	main, _ := newCancelTestAgent(t)
	coord := newBusyTestCoordinator(main)

	t.Cleanup(coord.trackSubSession("busy-child", "busy-parent"))
	t.Cleanup(coord.trackSubSession("idle-child", "idle-parent"))

	accept := main.BeginAccepted("busy-parent")
	t.Cleanup(accept.Close)

	require.True(t, coord.HasPendingWork("busy-child"))
	require.False(t, coord.HasPendingWork("idle-child"),
		"another parent's run must not mark this sub-session busy")
}

// Resolving a session must never read the database: the answer is wanted
// once per session in list endpoints and on keystrokes, and the common
// case is a top-level session that would miss the lookup and query for
// nothing.
func TestCoordinatorHasPendingWork_NeedsNoSessionService(t *testing.T) {
	t.Parallel()
	main, _ := newCancelTestAgent(t)
	coord := newBusyTestCoordinator(main)
	require.Nil(t, coord.sessions, "no session service wired")

	t.Cleanup(coord.trackSubSession("child", "parent"))
	accept := main.BeginAccepted("parent")
	t.Cleanup(accept.Close)

	require.True(t, coord.HasPendingWork("child"))
	require.False(t, coord.HasPendingWork("unrelated"))
}
