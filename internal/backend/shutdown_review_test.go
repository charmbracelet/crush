package backend

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// PROBE 4a: signal path = ShutdownWorkspaces() leaves the workspace registered;
// the SSE stream of an attached client then drops while the HTTP server shuts
// down -> DetachClient -> teardown() sees the workspace still registered and
// shuts it down AGAIN.
func TestShutdownWorkspacesThenDetachTearsDownOnce(t *testing.T) {
	b, _ := newTestBackend(t)
	ws, wsDown := insertTestWorkspace(t, b, t.TempDir())
	cid := newClientID(t)
	b.registerClient(ws, cid)
	require.NoError(t, b.AttachClient(ws.ID, cid))

	b.ShutdownWorkspaces() // what internal/cmd/server.go does on SIGTERM
	require.EqualValues(t, 1, wsDown.Load())

	b.DetachClient(ws.ID, cid) // SSE handler returns once the server goes down
	time.Sleep(100 * time.Millisecond)
	t.Logf("workspace shutdown count after ShutdownWorkspaces + DetachClient = %d", wsDown.Load())
	require.EqualValues(t, 1, wsDown.Load(), "workspace torn down twice")
}

// PROBE 4b: Backend.closing is not latched on the signal path, so a create that
// arrives after ShutdownWorkspaces is accepted by a server that is exiting.
func TestShutdownWorkspacesLatchesClosing(t *testing.T) {
	b, _ := newTestBackend(t)
	insertTestWorkspace(t, b, t.TempDir())
	b.ShutdownWorkspaces()
	b.mu.Lock()
	closing := b.closing
	b.mu.Unlock()
	t.Logf("b.closing after ShutdownWorkspaces = %v", closing)
	require.True(t, closing, "creates are still admitted after ShutdownWorkspaces")
}
