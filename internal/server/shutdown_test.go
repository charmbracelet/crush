package server

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
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

// TestListenAndServe_ShutsDownWhenNeverUsed guards against a spawned server
// whose client left before creating a workspace running forever.
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
		t.Fatal("a server that never hosted a workspace did not shut down")
	}
}

// TestHealth_UnavailableWhileClosing guards against an exiting server
// reporting healthy, so clients take it for its replacement.
func TestHealth_UnavailableWhileClosing(t *testing.T) {
	t.Parallel()
	srv := NewServer(nil, "tcp", "127.0.0.1:0")
	base, _ := serveOnLoopback(t, srv)
	t.Cleanup(func() { _ = srv.Close() })

	require.NoError(t, srv.Backend().ShutdownWorkspaces(t.Context()))
	code, err := sendRequest(t.Context(), base, http.MethodGet, "/v1/health", "")
	require.NoError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, code)
}

// TestServe_WaitsForSelfShutdown guards against Serve returning, and so
// `crush server` exiting, while a self-initiated shutdown is still tearing
// down a workspace.
func TestServe_WaitsForSelfShutdown(t *testing.T) {
	t.Parallel()
	srv := NewServer(nil, "tcp", "127.0.0.1:0")
	srv.backend.SetIdleShutdownDelay(time.Minute)
	gate := make(chan struct{})
	var openGate sync.Once
	t.Cleanup(func() { openGate.Do(func() { close(gate) }) })
	ws, cid := addHeldWorkspace(t, srv, func() { <-gate })
	_, other := addHeldWorkspace(t, srv, func() {})
	base, served := serveOnLoopback(t, srv)

	retired := make(chan error, 1)
	go func() {
		_, err := sendRequest(t.Context(), base, http.MethodDelete, "/v1/clients/"+cid, "")
		retired <- err
	}()
	require.Eventually(t, func() bool {
		_, err := srv.backend.GetWorkspace(ws.ID)
		return err != nil
	}, 5*time.Second, 10*time.Millisecond)
	code, err := sendRequest(t.Context(), base, http.MethodDelete, "/v1/clients/"+other, "")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, code)
	code, err = sendRequest(t.Context(), base, http.MethodPost, "/v1/control", `{"command":"shutdown_if_idle"}`)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, code)

	select {
	case err := <-served:
		t.Fatalf("Serve returned (%v) while a teardown was still running", err)
	case <-time.After(500 * time.Millisecond):
	}
	openGate.Do(func() { close(gate) })
	require.NoError(t, <-retired)
	require.ErrorIs(t, <-served, ErrServerClosed)
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

// TestShutdown_CancelsInFlightRequests guards against Shutdown waiting out
// a running '!' shell command instead of cancelling its request.
func TestShutdown_CancelsInFlightRequests(t *testing.T) {
	t.Parallel()
	srv := NewServer(nil, "tcp", "127.0.0.1:0")
	base, _ := serveOnLoopback(t, srv)

	// The handler persists the command's output.
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

	command := "touch started && sleep 30"
	if runtime.GOOS == "windows" {
		command = "touch started && ping -n 30 127.0.0.1"
	}
	body, err := json.Marshal(proto.ShellCommandRequest{Command: command})
	require.NoError(t, err)
	replied := make(chan error, 1)
	go func() {
		_, err := sendRequest(t.Context(), base, http.MethodPost,
			"/v1/workspaces/"+ws.ID+"/agent/sessions/"+uuid.NewString()+"/shell", string(body))
		replied <- err
	}()
	require.Eventually(t, func() bool {
		_, err := os.Stat(filepath.Join(ws.Path, "started"))
		return err == nil
	}, 10*time.Second, 10*time.Millisecond, "the shell command never started")

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	require.NoError(t, srv.Shutdown(ctx))
	select {
	case err := <-replied:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("the shell request did not return after shutdown")
	}
}
