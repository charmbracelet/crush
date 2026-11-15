package agent

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func newBusyTestCoordinator(main SessionAgent) *coordinator {
	return &coordinator{
		mainAgent:     main,
		mainAgentName: "coder",
		agents:        map[string]SessionAgent{"coder": main},
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

// Run holds its own accept while asking whether to queue, so HasActiveTurn
// must not count accepts or a run would queue behind itself.
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

// Summarize rewrites the transcript, so an accepted run about to start
// must block it just like an active one.
func TestSummarize_RefusesWhileARunIsAccepted(t *testing.T) {
	t.Parallel()
	sa, _ := newCancelTestAgent(t)

	accept := sa.BeginAccepted("sid")
	t.Cleanup(accept.Close)

	err := sa.Summarize(t.Context(), "sid", nil, nil)
	require.ErrorIs(t, err, ErrSessionBusy)
}

// Shutdown and agent switching key off IsBusy, so an accepted run counts.
func TestIsBusy_CoversAcceptedRuns(t *testing.T) {
	t.Parallel()
	sa, _ := newCancelTestAgent(t)

	require.False(t, sa.IsBusy())
	accept := sa.BeginAccepted("sid")
	require.True(t, sa.IsBusy())
	accept.Close()
	require.False(t, sa.IsBusy())
}

// A sub-agent runs on its own agent instance, so the main agent knows
// nothing of its session. trackSubSession registers it as live.
func TestCoordinatorIsSessionBusy_LiveSubSession(t *testing.T) {
	t.Parallel()
	main, _ := newCancelTestAgent(t)
	coord := newBusyTestCoordinator(main)

	require.False(t, coord.IsSessionBusy("child"))

	forget := coord.trackSubSession("child")
	require.True(t, coord.IsSessionBusy("child"),
		"a live sub-session is busy even when no parent run is registered")
	require.False(t, coord.IsSessionBusy("other"))

	forget()
	require.False(t, coord.IsSessionBusy("child"))
}

// Top-level sessions fall through to the agent, accepts included.
func TestCoordinatorIsSessionBusy_TopLevelSession(t *testing.T) {
	t.Parallel()
	main, _ := newCancelTestAgent(t)
	coord := newBusyTestCoordinator(main)

	require.False(t, coord.IsSessionBusy("solo"))
	accept := main.BeginAccepted("solo")
	t.Cleanup(accept.Close)
	require.True(t, coord.IsSessionBusy("solo"))
}

// Resolving a session must never read the database: it runs per session
// in list endpoints and on keystrokes.
func TestCoordinatorIsSessionBusy_NeedsNoSessionService(t *testing.T) {
	t.Parallel()
	main, _ := newCancelTestAgent(t)
	coord := newBusyTestCoordinator(main)
	require.Nil(t, coord.sessions, "no session service wired")

	forget := coord.trackSubSession("child")
	defer forget()
	require.True(t, coord.IsSessionBusy("child"))
	require.False(t, coord.IsSessionBusy("unrelated"))
}
