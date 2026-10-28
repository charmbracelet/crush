package backend

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// sessionRunCoordinator is an agent.Coordinator whose runs are contexts
// registered per session. Cancel ends a session's run the way the real
// coordinator cancels its active request, so a test can park a run on a
// permission prompt and see whether anything cancels it.
type sessionRunCoordinator struct {
	*blockingCoordinator

	mu   sync.Mutex
	runs map[string]context.CancelFunc
}

func (c *sessionRunCoordinator) Cancel(sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if cancel, ok := c.runs[sessionID]; ok {
		cancel()
	}
}

// startRun parks a run of sessionID on a permission prompt, which is
// where a run waits while its tool needs the user's answer. It returns
// the run's context and a channel that yields the prompt's error once
// the run stops waiting.
func (c *sessionRunCoordinator) startRun(t *testing.T, ws *Workspace, sessionID string) (context.Context, <-chan error) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	c.mu.Lock()
	c.runs[sessionID] = cancel
	c.mu.Unlock()

	done := make(chan error, 1)
	go func() {
		_, err := ws.Permissions.Request(ctx, permission.CreatePermissionRequest{
			SessionID:  sessionID,
			ToolCallID: "tc-" + sessionID,
			ToolName:   "bash",
			Action:     "execute",
			Path:       ws.Path,
		})
		done <- err
	}()
	require.Eventually(t, func() bool {
		req, ok := ws.Permissions.ActiveRequest()
		return ok && req.SessionID == sessionID
	}, 10*time.Second, 5*time.Millisecond, "the run of %s must be waiting on its prompt", sessionID)
	return ctx, done
}

// newRunWorkspace installs a workspace with a real permission service and
// a sessionRunCoordinator. Its teardown is stubbed out and cancels
// nothing, so a cancelled run can only have been cancelled by a client
// leaving.
func newRunWorkspace(t *testing.T, b *Backend) (*Workspace, *sessionRunCoordinator) {
	t.Helper()
	a := app.NewForTest(context.Background())
	t.Cleanup(a.ShutdownForTest)
	coord := &sessionRunCoordinator{
		blockingCoordinator: newBlockingCoordinator(),
		runs:                make(map[string]context.CancelFunc),
	}
	a.AgentCoordinator = coord
	ws := &Workspace{ID: uuid.New().String(), Path: t.TempDir(), App: a}
	SetWorkspaceShutdownFnForTest(ws, func() {})
	InsertWorkspaceForTest(b, ws)
	return ws, coord
}

// attachViewing attaches clientID to ws with a live stream and reports it
// as viewing sessionID, as the TUI does once it opens a session.
func attachViewing(t *testing.T, b *Backend, ws *Workspace, clientID, sessionID string) {
	t.Helper()
	require.NoError(t, b.AttachClient(ws.ID, clientID))
	require.NoError(t, b.SetCurrentSession(t.Context(), ws.ID, clientID, sessionID))
}

// awaitCancelled waits for a parked run to stop waiting on its prompt and
// checks that it stopped because it was cancelled.
func awaitCancelled(t *testing.T, done <-chan error, msg string) {
	t.Helper()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled, msg)
	case <-time.After(10 * time.Second):
		t.Fatal(msg)
	}
}

// TestClientDeparture_CancelsRunNobodyViews is the regression for a client
// that quits or is killed while its run waits on a permission prompt and
// another client keeps the workspace alive. Prompts only reach the clients
// viewing their session, so nobody was left to answer that one: the run
// waited headless forever, and since only one prompt is shown at a time,
// the remaining client's own prompts never appeared either. However the
// departing client's claim ends, the run of the session it was viewing
// must be cancelled, as quitting crush does in local mode.
func TestClientDeparture_CancelsRunNobodyViews(t *testing.T) {
	t.Parallel()

	departures := map[string]func(t *testing.T, b *Backend, ws *Workspace, clientID string){
		"retired": func(t *testing.T, b *Backend, _ *Workspace, clientID string) {
			require.NoError(t, b.RetireClient(clientID))
		},
		"detach grace expired": func(t *testing.T, b *Backend, ws *Workspace, clientID string) {
			b.SetDetachGrace(50 * time.Millisecond)
			b.DetachClient(ws.ID, clientID)
		},
		"detach grace disabled": func(t *testing.T, b *Backend, ws *Workspace, clientID string) {
			b.SetDetachGrace(0)
			b.DetachClient(ws.ID, clientID)
		},
		"released then detached": func(t *testing.T, b *Backend, ws *Workspace, clientID string) {
			b.SetDetachGrace(time.Hour)
			require.NoError(t, b.DeleteWorkspace(ws.ID, clientID))
			b.DetachClient(ws.ID, clientID)
		},
		"detached then released": func(t *testing.T, b *Backend, ws *Workspace, clientID string) {
			b.SetDetachGrace(time.Hour)
			b.DetachClient(ws.ID, clientID)
			require.NoError(t, b.DeleteWorkspace(ws.ID, clientID))
		},
	}
	for name, depart := range departures {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b, _ := newTestBackend(t)
			ws, coord := newRunWorkspace(t, b)

			quitter, stayer := newClientID(t), newClientID(t)
			attachViewing(t, b, ws, quitter, "s-quitter")
			attachViewing(t, b, ws, stayer, "s-stayer")
			_, done := coord.startRun(t, ws, "s-quitter")

			depart(t, b, ws, quitter)

			awaitCancelled(t, done,
				"the run of a session no remaining client views must be cancelled when its viewer leaves")
			_, err := b.GetWorkspace(ws.ID)
			require.NoError(t, err, "the remaining client keeps the workspace")

			// The remaining client's prompts are no longer held back.
			coord.startRun(t, ws, "s-stayer")
		})
	}
}

// TestClientDeparture_KeepsRunAnotherClientViews is the other side: a run
// is only cancelled once nobody is left who could answer it. A client
// still viewing the session keeps the run going, even while that client
// waits out its detach grace, since it is expected back.
func TestClientDeparture_KeepsRunAnotherClientViews(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	b.SetDetachGrace(time.Hour)
	ws, coord := newRunWorkspace(t, b)

	quitter, watcher, bystander := newClientID(t), newClientID(t), newClientID(t)
	attachViewing(t, b, ws, quitter, "s-shared")
	attachViewing(t, b, ws, watcher, "s-shared")
	require.NoError(t, b.AttachClient(ws.ID, bystander))
	run, done := coord.startRun(t, ws, "s-shared")

	// The watcher's stream drops; it may still reconnect.
	b.DetachClient(ws.ID, watcher)
	require.NoError(t, b.RetireClient(quitter))
	require.NoError(t, run.Err(), "a run another client still views must keep going")

	require.NoError(t, b.RetireClient(watcher))
	awaitCancelled(t, done, "the run must be cancelled once its last viewer leaves")
}
