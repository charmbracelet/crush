package backend

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// busyCoordinator reports an active run until busy is cleared, and counts
// session cancels.
type busyCoordinator struct {
	*blockingCoordinator
	busy    atomic.Bool
	cancels atomic.Int32
}

func (c *busyCoordinator) IsBusy() bool  { return c.busy.Load() }
func (c *busyCoordinator) Cancel(string) { c.cancels.Add(1) }

// newBusyBackend returns a backend that re-checks deferred teardowns every
// 10ms, plus a workspace on it whose coordinator reports a run in flight.
func newBusyBackend(t *testing.T, orphanTimeout time.Duration) (b *Backend, ws *Workspace, coord *busyCoordinator, wsDown, srvDown *atomic.Int32) {
	t.Helper()
	b, srvDown = newTestBackend(t)
	b.busyRecheck = 10 * time.Millisecond
	b.orphanRunTimeout = orphanTimeout
	b.SetDetachGrace(20 * time.Millisecond)
	ws, wsDown = insertTestWorkspace(t, b, "/tmp/busy-"+uuid.NewString())
	coord = &busyCoordinator{blockingCoordinator: newBlockingCoordinator()}
	coord.busy.Store(true)
	// A run left busy would keep re-deferring the teardown after the test.
	t.Cleanup(func() { coord.busy.Store(false) })
	ws.App = &app.App{AgentCoordinator: coord}
	return b, ws, coord, wsDown, srvDown
}

func teardownDeferred(ws *Workspace) bool {
	ws.clientsMu.Lock()
	defer ws.clientsMu.Unlock()
	return ws.teardownRecheck != nil
}

// TestTeardown_DefersWhileRunActive is the core of leaving the TUI without
// killing work: once the last client's claim is gone (here after the detach
// grace a crashed client gets), a workspace whose run is still active must
// stay up, and the server with it, until the run ends.
func TestTeardown_DefersWhileRunActive(t *testing.T) {
	t.Parallel()

	b, ws, coord, wsDown, srvDown := newBusyBackend(t, time.Hour)
	cid := newClientID(t)
	b.registerClient(ws, cid)
	require.NoError(t, b.AttachClient(ws.ID, cid))
	b.DetachClient(ws.ID, cid)

	require.Never(t, func() bool { return wsDown.Load() > 0 },
		300*time.Millisecond, 10*time.Millisecond,
		"an active run must keep its workspace alive after the last client leaves")
	ws.clientsMu.Lock()
	claims := len(ws.clients)
	ws.clientsMu.Unlock()
	require.Zero(t, claims, "the client's claim must have expired; only the run holds the workspace")
	_, err := b.GetWorkspace(ws.ID)
	require.NoError(t, err)
	require.Zero(t, srvDown.Load(), "the server must not idle-shut down while a run is active")

	coord.busy.Store(false)
	require.Eventually(t, func() bool { return wsDown.Load() == 1 && srvDown.Load() == 1 },
		2*time.Second, 10*time.Millisecond,
		"once the run ends the unclaimed workspace and the idle server go away")
	_, err = b.GetWorkspace(ws.ID)
	require.ErrorIs(t, err, ErrWorkspaceNotFound)
}

// TestTeardown_DefersWhileDispatchedRunInFlight covers runs the
// coordinator's IsBusy cannot see: one accepted but not started yet, or one
// still running on a coordinator that a later agent init replaced. A run
// dispatched through SendMessage keeps the workspace until it returns.
func TestTeardown_DefersWhileDispatchedRunInFlight(t *testing.T) {
	t.Parallel()

	b, srvDown := newTestBackend(t)
	b.busyRecheck = 10 * time.Millisecond
	b.orphanRunTimeout = time.Hour
	coord := newBlockingCoordinator() // IsBusy reports false throughout.
	ws := insertAgentWorkspace(t, b, coord)
	var wsDown atomic.Int32
	ws.shutdownFn = func() { wsDown.Add(1) }

	cid := newClientID(t)
	require.NoError(t, b.AttachClient(ws.ID, cid))
	require.NoError(t, b.SendMessage(ws.ID, proto.AgentMessage{SessionID: "S1", Prompt: "hi"}))
	b.DetachClient(ws.ID, cid)

	require.Never(t, func() bool { return wsDown.Load() > 0 },
		200*time.Millisecond, 10*time.Millisecond,
		"a dispatched run must keep its workspace alive after the last client leaves")

	close(coord.release)
	require.Eventually(t, func() bool { return wsDown.Load() == 1 && srvDown.Load() == 1 },
		2*time.Second, 10*time.Millisecond,
		"once the run returns the unclaimed workspace is torn down")
}

// TestTeardown_ClientReturnCancelsDeferral checks that a client coming back
// while a teardown is deferred takes the workspace over again: the pending
// re-check and the orphan clock are cleared, and the run ending later no
// longer tears it down from under that client.
func TestTeardown_ClientReturnCancelsDeferral(t *testing.T) {
	t.Parallel()

	arrivals := map[string]func(b *Backend, ws *Workspace, cid string){
		"attach": func(b *Backend, ws *Workspace, cid string) { require.NoError(t, b.AttachClient(ws.ID, cid)) },
		"create": func(b *Backend, ws *Workspace, cid string) { b.registerClient(ws, cid) },
	}
	for name, arrive := range arrivals {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			b, ws, coord, wsDown, _ := newBusyBackend(t, time.Hour)
			cid := newClientID(t)
			require.NoError(t, b.AttachClient(ws.ID, cid))
			b.DetachClient(ws.ID, cid)
			require.Eventually(t, func() bool { return teardownDeferred(ws) },
				2*time.Second, 5*time.Millisecond, "teardown must be deferred while the run is active")

			arrive(b, ws, newClientID(t))
			ws.clientsMu.Lock()
			recheck, orphanedAt := ws.teardownRecheck, ws.orphanedAt
			ws.clientsMu.Unlock()
			require.Nil(t, recheck, "a returning client must cancel the pending re-check")
			require.True(t, orphanedAt.IsZero(), "a returning client must reset the orphan clock")

			if name == "attach" {
				coord.busy.Store(false)
				require.Never(t, func() bool { return wsDown.Load() > 0 },
					200*time.Millisecond, 10*time.Millisecond,
					"an attached client keeps the workspace after the run ends")
			}
		})
	}
}

// TestTeardown_OrphanTimeoutTearsDownStuckRun covers a run that can never
// finish without a client (blocked on a permission or question prompt): the
// deferral is capped so it cannot pin the workspace and server forever.
func TestTeardown_OrphanTimeoutTearsDownStuckRun(t *testing.T) {
	t.Parallel()

	const timeout = 100 * time.Millisecond
	b, ws, _, wsDown, srvDown := newBusyBackend(t, timeout)
	cid := newClientID(t)
	require.NoError(t, b.AttachClient(ws.ID, cid))
	start := time.Now()
	b.DetachClient(ws.ID, cid)

	require.Eventually(t, func() bool { return wsDown.Load() == 1 && srvDown.Load() == 1 },
		2*time.Second, 5*time.Millisecond,
		"a run still busy past the orphan timeout must not keep the workspace")
	require.GreaterOrEqual(t, time.Since(start), timeout,
		"the workspace must have outlived its client for the orphan timeout")
}

// TestTeardown_ImmediateWithoutDeferral pins the unchanged paths: no active
// run, or the deferral disabled (orphan timeout 0), tears the workspace
// down as soon as the last claim goes.
func TestTeardown_ImmediateWithoutDeferral(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		busy    bool
		timeout time.Duration
	}{
		"idle run":          {busy: false, timeout: time.Hour},
		"deferral disabled": {busy: true, timeout: 0},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			b, ws, coord, wsDown, _ := newBusyBackend(t, tc.timeout)
			coord.busy.Store(tc.busy)
			b.SetDetachGrace(0)
			cid := newClientID(t)
			require.NoError(t, b.AttachClient(ws.ID, cid))
			b.DetachClient(ws.ID, cid)

			require.Equal(t, int32(1), wsDown.Load(), "teardown must run synchronously as before")
			require.False(t, teardownDeferred(ws))
		})
	}
}

// TestTeardown_StaleRecheckIsNoop guards the deferral's re-check timer: one
// that fires after another path already tore the workspace down must not
// shut its app down a second time.
func TestTeardown_StaleRecheckIsNoop(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	ws, wsDown := insertTestWorkspace(t, b, "/tmp/stale-recheck")

	b.teardown(ws)
	b.teardown(ws)

	require.Equal(t, int32(1), wsDown.Load(), "a workspace must be shut down exactly once")
}

// TestTeardown_KeepsRunOfReplacedCoordinator is the case measured in the
// E2E log: an agent init swaps in a fresh coordinator while a run is still
// going on the old one, so the workspace's IsBusy reports false. The run
// must still keep the workspace until it returns.
func TestTeardown_KeepsRunOfReplacedCoordinator(t *testing.T) {
	t.Parallel()

	b, srvDown := newTestBackend(t)
	b.busyRecheck = 10 * time.Millisecond
	b.orphanRunTimeout = time.Hour
	old := &busyCoordinator{blockingCoordinator: newBlockingCoordinator()}
	old.busy.Store(true)
	ws := insertAgentWorkspace(t, b, old)
	var wsDown atomic.Int32
	ws.shutdownFn = func() { wsDown.Add(1) }

	cid := newClientID(t)
	require.NoError(t, b.AttachClient(ws.ID, cid))
	require.NoError(t, b.SendMessage(ws.ID, proto.AgentMessage{SessionID: "S1", Prompt: "hi"}))
	<-old.entered
	ws.AgentCoordinator = &busyCoordinator{blockingCoordinator: newBlockingCoordinator()} // Idle.
	b.DetachClient(ws.ID, cid)

	require.Never(t, func() bool { return wsDown.Load() > 0 },
		200*time.Millisecond, 10*time.Millisecond,
		"a run on a replaced coordinator must keep its workspace alive")

	old.busy.Store(false)
	close(old.release)
	require.Eventually(t, func() bool { return wsDown.Load() == 1 && srvDown.Load() == 1 },
		2*time.Second, 10*time.Millisecond,
		"once the old coordinator's run returns the workspace is torn down")
}

// TestOrphanRunTimeoutFromEnv pins CRUSH_SERVER_ORPHAN_RUN_TIMEOUT: 0 turns
// the deferral off (the old tear-down-at-once behaviour), and an invalid
// value falls back to the default with a warning.
func TestOrphanRunTimeoutFromEnv(t *testing.T) {
	cases := []struct {
		val  string
		want time.Duration
		warn bool
	}{
		{"", DefaultOrphanRunTimeout, false},
		{"0", 0, false},
		{"90", 90 * time.Second, false},
		{"abc", DefaultOrphanRunTimeout, true},
		{"-5", DefaultOrphanRunTimeout, true},
	}
	for _, tc := range cases {
		t.Run(tc.val, func(t *testing.T) {
			t.Setenv("CRUSH_SERVER_ORPHAN_RUN_TIMEOUT", tc.val)
			logs := captureDebugLogs(t)
			b := New(t.Context(), nil, nil)
			require.Equal(t, tc.want, b.orphanRunTimeout)
			require.Equal(t, tc.warn, strings.Contains(logs.String(), "CRUSH_SERVER_ORPHAN_RUN_TIMEOUT"),
				"warning expected: %v; logs: %s", tc.warn, logs.String())

			if tc.want == 0 {
				ws, wsDown := insertTestWorkspace(t, b, "/tmp/orphan-env-zero")
				coord := &busyCoordinator{blockingCoordinator: newBlockingCoordinator()}
				coord.busy.Store(true)
				ws.App = &app.App{AgentCoordinator: coord}
				b.teardown(ws)
				require.Equal(t, int32(1), wsDown.Load(), "0 tears a busy workspace down at once")
			}
		})
	}
}
