package server

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/backend"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestShutdown_TearsDownHostedWorkspaces covers the path a termination
// signal takes: stopping the server must tear down live workspaces so
// their interrupted work is recorded.
func TestShutdown_TearsDownHostedWorkspaces(t *testing.T) {
	t.Parallel()

	srv := NewServer(nil, "tcp", "127.0.0.1:0")
	var tornDown atomic.Int32
	ws := &backend.Workspace{ID: uuid.New().String(), Path: t.TempDir()}
	backend.SetWorkspaceShutdownFnForTest(ws, func() { tornDown.Add(1) })
	backend.InsertWorkspaceForTest(srv.backend, ws)

	require.NoError(t, srv.Shutdown(t.Context()))
	require.Equal(t, int32(1), tornDown.Load())
}

// TestListenAndServe_ShutsDownWhenNeverUsed covers a server spawned for a
// client that went away before creating a workspace (interrupted, or
// failed first): with nothing ever hosted, it must still exit once the
// idle linger elapses instead of running forever.
func TestListenAndServe_ShutsDownWhenNeverUsed(t *testing.T) {
	t.Parallel()

	srv := NewServer(nil, "tcp", "127.0.0.1:0")
	srv.Backend().SetIdleShutdownDelay(100 * time.Millisecond)

	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	t.Cleanup(func() { _ = srv.Close() })

	select {
	case err := <-errc:
		require.ErrorIs(t, err, ErrServerClosed)
	case <-time.After(5 * time.Second):
		t.Fatal("a server that never hosted a workspace is still running " +
			"long after its idle linger elapsed")
	}
}

// TestHealth_UnavailableWhileClosing covers the readiness probe clients
// use to tell whether a server is up. A server that has committed to
// exiting refuses new workspaces, so it must not report itself healthy:
// a client waiting for a replacement would take it for one.
func TestHealth_UnavailableWhileClosing(t *testing.T) {
	t.Parallel()

	srv := NewServer(nil, "tcp", "127.0.0.1:0")
	hs := httptest.NewServer(srv.Handler())
	t.Cleanup(hs.Close)

	health := func() int {
		t.Helper()
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, hs.URL+"/v1/health", nil)
		require.NoError(t, err)
		rsp, err := hs.Client().Do(req)
		require.NoError(t, err)
		defer rsp.Body.Close()
		return rsp.StatusCode
	}
	require.Equal(t, http.StatusOK, health())

	// A termination signal latches the decision to exit, then keeps
	// serving while it tears the hosted workspaces down.
	require.NoError(t, srv.Backend().ShutdownWorkspaces(t.Context()))
	require.Equal(t, http.StatusServiceUnavailable, health())
}
