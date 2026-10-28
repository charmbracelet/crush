package cmd

import (
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/client"
	"github.com/charmbracelet/crush/internal/server"
	"github.com/stretchr/testify/require"
)

// serveTCP starts a real server on a loopback port.
func serveTCP(t *testing.T) (*server.Server, *url.URL) {
	t.Helper()
	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := server.NewServer(nil, "tcp", ln.Addr().String())
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return srv, &url.URL{Scheme: "tcp", Host: ln.Addr().String()}
}

// requireWaitsForRelease checks that awaitExitingServerGone keeps waiting
// while srv, which has committed to exiting, still holds hostURL, and
// stops once srv lets go of it.
func requireWaitsForRelease(t *testing.T, srv *server.Server, hostURL *url.URL) {
	t.Helper()
	require.NoError(t, srv.Backend().ShutdownWorkspaces(t.Context()))

	done := make(chan error, 1)
	go func() { done <- awaitExitingServerGone(t.Context(), hostURL) }()

	select {
	case err := <-done:
		t.Fatalf("stopped waiting while the exiting server still held the address: %v", err)
	case <-time.After(500 * time.Millisecond):
	}

	require.NoError(t, srv.Close())
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("kept waiting after the exiting server released the address")
	}
}

// TestAwaitExitingServerGone_WaitsForRelease covers replacing a server
// that has committed to exiting when it listens on tcp (or, on Windows, a
// named pipe). Neither address can be taken over while that server holds
// it, so a replacement spawned any earlier would fail to bind.
func TestAwaitExitingServerGone_WaitsForRelease(t *testing.T) {
	t.Parallel()

	srv, hostURL := serveTCP(t)
	requireWaitsForRelease(t, srv, hostURL)
}

// TestAwaitExitingServerGone_AcceptsReplacement covers a client that lost
// the race to replace the exiting server: a server that is not shutting
// down already answers, so there is nothing left to wait for.
func TestAwaitExitingServerGone_AcceptsReplacement(t *testing.T) {
	t.Parallel()

	_, hostURL := serveTCP(t)
	require.NoError(t, awaitExitingServerGone(t.Context(), hostURL))
}

// TestWaitForServerReady_StopsAtExitingServer covers a client that finds
// a server which has committed to exiting. That server will never become
// ready, so waiting on it must end at once, letting the caller replace
// it, instead of outlasting it and failing with nothing left to connect
// to.
func TestWaitForServerReady_StopsAtExitingServer(t *testing.T) {
	t.Parallel()

	srv, hostURL := serveTCP(t)
	require.NoError(t, srv.Backend().ShutdownWorkspaces(t.Context()))

	start := time.Now()
	err := waitForServerReady(t.Context(), hostURL)
	require.ErrorIs(t, err, client.ErrServerShuttingDown)
	require.Less(t, time.Since(start), 5*time.Second,
		"waited on a server that will never become ready")
}
