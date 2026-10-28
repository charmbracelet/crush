package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/backend"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/proto"
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

// TestServe_WaitsForSelfShutdown covers a server that stops itself while
// a workspace is still being torn down inside its exiting client's
// DELETE /v1/clients request. `crush server` exits as soon as Serve
// returns, so Serve must wait for that shutdown to finish rather than
// return the moment the listener closes, which would kill the teardown
// and leave the client without a reply.
func TestServe_WaitsForSelfShutdown(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		linger time.Duration
		// stop retires other, the client of a second workspace, and makes
		// the server stop itself while the first teardown is blocked.
		stop func(t *testing.T, base, other string)
	}{
		{
			name:   "client asks for shutdown_if_idle",
			linger: time.Minute,
			stop: func(t *testing.T, base, other string) {
				code, err := sendRequest(t.Context(), base, http.MethodDelete, "/v1/clients/"+other, "")
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, code)
				code, err = sendRequest(t.Context(), base, http.MethodPost, "/v1/control", `{"command":"shutdown_if_idle"}`)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, code)
			},
		},
		{
			name:   "last client leaves with lingering disabled",
			linger: 0,
			stop: func(t *testing.T, base, other string) {
				code, err := sendRequest(t.Context(), base, http.MethodDelete, "/v1/clients/"+other, "")
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, code)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			srv := NewServer(nil, "tcp", "127.0.0.1:0")
			srv.backend.SetIdleShutdownDelay(tc.linger)
			gate := make(chan struct{})
			var openGate sync.Once
			t.Cleanup(func() { openGate.Do(func() { close(gate) }) })
			ws, cid := addHeldWorkspace(t, srv, func() { <-gate })
			_, other := addHeldWorkspace(t, srv, func() {})
			base, served := serveOnLoopback(t, srv)

			retired := make(chan error, 1)
			go func() {
				code, err := sendRequest(t.Context(), base, http.MethodDelete, "/v1/clients/"+cid, "")
				if err == nil && code != http.StatusOK {
					err = fmt.Errorf("unexpected status %d", code)
				}
				retired <- err
			}()
			require.Eventually(t, func() bool {
				_, err := srv.backend.GetWorkspace(ws.ID)
				return err != nil
			}, 5*time.Second, 10*time.Millisecond, "the retire must start the teardown")

			tc.stop(t, base, other)

			select {
			case err := <-served:
				t.Fatalf("Serve returned (%v) while a workspace teardown was still running", err)
			case <-time.After(500 * time.Millisecond):
			}

			openGate.Do(func() { close(gate) })
			select {
			case err := <-retired:
				require.NoError(t, err, "the retiring client must get its reply")
			case <-time.After(5 * time.Second):
				t.Fatal("the retire request did not finish after its teardown did")
			}
			select {
			case err := <-served:
				require.ErrorIs(t, err, ErrServerClosed)
			case <-time.After(5 * time.Second):
				t.Fatal("Serve did not return after the shutdown finished")
			}

			// The backend can ask again, as an idle timer firing late
			// would. That must not start a second shutdown.
			require.True(t, srv.backend.ShutdownIfIdle())
		})
	}
}

// TestServe_SelfShutdownIsBounded covers a teardown that never finishes.
// Serve waits for the shutdown the server starts on its own, so that
// shutdown must give up after its budget and close what is left rather
// than keep the server up for good.
func TestServe_SelfShutdownIsBounded(t *testing.T) {
	t.Parallel()

	srv := NewServer(nil, "tcp", "127.0.0.1:0")
	gate := make(chan struct{})
	t.Cleanup(func() { close(gate) })
	ws, cid := addHeldWorkspace(t, srv, func() { <-gate })
	base, served := serveOnLoopback(t, srv)

	retired := make(chan error, 1)
	go func() {
		_, err := sendRequest(t.Context(), base, http.MethodDelete, "/v1/clients/"+cid, "")
		retired <- err
	}()
	require.Eventually(t, func() bool {
		_, err := srv.backend.GetWorkspace(ws.ID)
		return err != nil
	}, 5*time.Second, 10*time.Millisecond, "the retire must start the teardown")

	srv.shutdownSelf(100 * time.Millisecond)

	select {
	case err := <-served:
		require.ErrorIs(t, err, ErrServerClosed)
	case <-time.After(5 * time.Second):
		t.Fatal("Serve kept waiting for a shutdown that cannot finish")
	}
	select {
	case err := <-retired:
		require.Error(t, err, "the stuck request's connection must be closed")
	case <-time.After(5 * time.Second):
		t.Fatal("the stuck request's connection was left open")
	}
}

// addHeldWorkspace hosts a synthetic workspace on srv, held by a fresh
// client, whose teardown runs teardown. It returns the workspace and the
// client's ID.
func addHeldWorkspace(t *testing.T, srv *Server, teardown func()) (*backend.Workspace, string) {
	t.Helper()
	ws := &backend.Workspace{ID: uuid.New().String(), Path: t.TempDir()}
	backend.SetWorkspaceShutdownFnForTest(ws, teardown)
	backend.InsertWorkspaceForTest(srv.backend, ws)
	cid := uuid.New().String()
	require.NoError(t, backend.RegisterClientForTesting(srv.backend, ws, cid))
	return ws, cid
}

// serveOnLoopback runs srv.Serve on a loopback TCP listener. It returns
// the server's base URL and a channel that receives Serve's result.
func serveOnLoopback(t *testing.T, srv *Server) (string, <-chan error) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0") //nolint:noctx
	require.NoError(t, err)
	served := make(chan error, 1)
	go func() { served <- srv.Serve(ln) }()
	return "http://" + ln.Addr().String(), served
}

// sendRequest sends a request on a connection of its own and returns the
// response status.
func sendRequest(ctx context.Context, base, method, path, body string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, method, base+path, strings.NewReader(body))
	if err != nil {
		return 0, err
	}
	tr := &http.Transport{DisableKeepAlives: true}
	defer tr.CloseIdleConnections()
	res, err := (&http.Client{Transport: tr}).Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, res.Body)
	return res.StatusCode, nil
}

// TestShutdown_CancelsInFlightRequests covers a '!' shell command still
// running when the server stops. The command runs on its request's
// context, which the workspace teardown cannot reach, so Shutdown must
// cancel in-flight requests itself instead of waiting them out.
func TestShutdown_CancelsInFlightRequests(t *testing.T) {
	t.Parallel()

	// Serve on a real listener: an httptest server around Handler would
	// bypass the server's own http.Server.
	srv := NewServer(nil, "tcp", "127.0.0.1:0")
	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = srv.Serve(ln) }()

	// The handler persists the command's output, so the workspace needs
	// a message store.
	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	ws := &backend.Workspace{
		ID:   uuid.New().String(),
		Path: t.TempDir(),
		App:  &app.App{Messages: message.NewService(db.New(conn))},
	}
	backend.SetWorkspaceShutdownFnForTest(ws, func() {})
	backend.InsertWorkspaceForTest(srv.backend, ws)

	// Windows has no sleep command; ping waits about as long.
	command := "touch started && sleep 30"
	if runtime.GOOS == "windows" {
		command = "touch started && ping -n 30 127.0.0.1"
	}
	body, err := json.Marshal(proto.ShellCommandRequest{Command: command})
	require.NoError(t, err)
	url := fmt.Sprintf("http://%s/v1/workspaces/%s/agent/sessions/%s/shell",
		ln.Addr(), ws.ID, uuid.New())
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	replied := make(chan error, 1)
	go func() {
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
		}
		replied <- err
	}()

	require.Eventually(t, func() bool {
		_, err := os.Stat(filepath.Join(ws.Path, "started"))
		return err == nil
	}, 10*time.Second, 10*time.Millisecond, "the shell command never started")

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	require.NoError(t, srv.Shutdown(ctx),
		"shutdown must interrupt the running command, not wait it out")
	select {
	case err := <-replied:
		require.NoError(t, err, "the client must get a reply")
	case <-time.After(5 * time.Second):
		t.Fatal("the shell request did not return after shutdown")
	}
}
