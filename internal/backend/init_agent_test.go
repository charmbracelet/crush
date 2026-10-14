package backend

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestInitAgent_KeepsCoordinatorWhileRunsInFlight: every TUI connect asks
// for an agent init, and replacing the coordinator under a run orphans that
// run (cancels and busy probes only reach the new one). While a run is in
// flight the init must keep the existing coordinator.
func TestInitAgent_KeepsCoordinatorWhileRunsInFlight(t *testing.T) {
	t.Parallel()

	cases := map[string]func(t *testing.T, b *Backend, ws *Workspace, coord *busyCoordinator){
		"coordinator busy": func(_ *testing.T, _ *Backend, _ *Workspace, coord *busyCoordinator) {
			coord.busy.Store(true)
		},
		"run dispatched, not started yet": func(t *testing.T, b *Backend, ws *Workspace, _ *busyCoordinator) {
			require.NoError(t, b.SendMessage(ws.ID, proto.AgentMessage{SessionID: "S1", Prompt: "hi"}))
		},
	}
	for name, start := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			b, _ := newTestBackend(t)
			coord := &busyCoordinator{blockingCoordinator: newBlockingCoordinator()}
			ws := insertAgentWorkspace(t, b, coord)
			t.Cleanup(func() { close(coord.release) })
			start(t, b, ws, coord)

			// The synthetic workspace has no config, so a real init would
			// panic: getting past this line means it was skipped.
			require.NoError(t, b.InitAgent(t.Context(), ws.ID, true))
			require.Same(t, agentCoordinatorOf(ws), coord, "init must keep the coordinator running the work")

			if coord.busy.Load() {
				info, err := b.GetAgentInfo(ws.ID)
				require.NoError(t, err)
				require.True(t, info.IsBusy, "the agent must still report the run")
			}
			require.NoError(t, b.CancelSession(ws.ID, "S1"))
			require.Equal(t, int32(1), coord.cancels.Load(), "the run must stay cancellable")
		})
	}
}

// TestInitAgent_ReplacesCoordinatorWhenIdle keeps the existing behaviour
// when nothing runs: an init swaps in a fresh coordinator.
func TestInitAgent_ReplacesCoordinatorWhenIdle(t *testing.T) {
	b, ws := newConfiguredWorkspace(t)
	old := agentCoordinatorOf(ws)
	require.NotNil(t, old)

	require.NoError(t, b.InitAgent(t.Context(), ws.ID, true))
	require.NotSame(t, old, agentCoordinatorOf(ws), "an idle init must replace the coordinator")
}

// TestInitAgent_RacesDeferredTeardown runs an init (which replaces the
// coordinator) while a deferred teardown re-checks the same workspace on
// its timer, for the race detector.
func TestInitAgent_RacesDeferredTeardown(t *testing.T) {
	b, ws := newConfiguredWorkspace(t)
	b.busyRecheck = time.Millisecond
	b.orphanRunTimeout = time.Hour
	coord := &busyCoordinator{blockingCoordinator: newBlockingCoordinator()}
	coord.busy.Store(true)
	setAgentCoordinator(ws, coord)

	drainBackend(t, b) // The creator's claim goes; the busy run defers teardown.
	require.True(t, teardownDeferred(ws))

	coord.busy.Store(false)                   // The next re-check tears the workspace down,
	_ = b.InitAgent(t.Context(), ws.ID, true) // racing this replacement.
	require.Eventually(t, func() bool { _, err := b.GetWorkspace(ws.ID); return err != nil },
		5*time.Second, 5*time.Millisecond)
}

// hookedCoordinator runs onBegin inside BeginAccepted: the instant
// SendMessage takes a reservation on the coordinator, before the run starts.
type hookedCoordinator struct {
	*blockingCoordinator
	onBegin func()
}

func (c *hookedCoordinator) BeginAccepted(sessionID string) *agent.AcceptedRun {
	c.onBegin()
	return c.blockingCoordinator.BeginAccepted(sessionID)
}

// TestInitAgent_KeepsCoordinatorWhileRunIsBeingAccepted: an init landing
// while SendMessage accepts a prompt must not replace the coordinator that
// holds the run's reservation, or the run is orphaned like a running one.
func TestInitAgent_KeepsCoordinatorWhileRunIsBeingAccepted(t *testing.T) {
	b, ws := newConfiguredWorkspace(t)
	coord := &hookedCoordinator{blockingCoordinator: newBlockingCoordinator()}
	coord.onBegin = func() { require.NoError(t, b.InitAgent(t.Context(), ws.ID, true)) }
	setAgentCoordinator(ws, coord)
	t.Cleanup(func() { close(coord.release) })

	require.NoError(t, b.SendMessage(ws.ID, proto.AgentMessage{SessionID: "S1", Prompt: "hi"}))
	require.Same(t, coord, agentCoordinatorOf(ws), "init must keep the coordinator that accepted the run")
}

// newConfiguredWorkspace creates a real workspace whose config has a
// provider, so app.New starts a real coordinator. Not parallel: it sets
// environment variables.
func newConfiguredWorkspace(t *testing.T) (*Backend, *Workspace) {
	t.Helper()
	xdgIsolated(t)
	t.Setenv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE", "1")
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "crush.json"), []byte(`{
 "providers":{"fake":{"type":"openai-compat","base_url":"http://127.0.0.1:1/v1","api_key":"x",
  "models":[{"id":"m","name":"m","context_window":100000,"default_max_tokens":1000}]}},
 "models":{"large":{"provider":"fake","model":"m"},"small":{"provider":"fake","model":"m"}},
 "options":{"disable_provider_auto_update":true,"disable_metrics":true}}`), 0o600))

	b, _ := newTestBackend(t)
	b.SetCreateGrace(time.Hour)
	ws, _, err := b.CreateWorkspace(protoWS(dir, t.TempDir(), uuid.New().String()))
	require.NoError(t, err)
	t.Cleanup(func() { drainBackend(t, b) })
	return b, ws
}

// agentCoordinatorOf and setAgentCoordinator access the coordinator under
// runMu, the lock InitAgent replaces it under.
func agentCoordinatorOf(ws *Workspace) any {
	ws.runMu.Lock()
	defer ws.runMu.Unlock()
	return ws.AgentCoordinator
}

func setAgentCoordinator(ws *Workspace, c agent.Coordinator) {
	ws.runMu.Lock()
	defer ws.runMu.Unlock()
	ws.AgentCoordinator = c
}
