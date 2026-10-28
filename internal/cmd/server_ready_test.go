package cmd

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// exitedServer returns a spawnedServer whose process has already exited
// with err after writing stderr.
func exitedServer(t *testing.T, err error, stderr string) *spawnedServer {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stderr.log")
	require.NoError(t, os.WriteFile(path, []byte(stderr), 0o600))
	child := &spawnedServer{stderrPath: path, done: make(chan struct{}), err: err}
	close(child.done)
	return child
}

// TestWaitForServerReady_FailsFastWhenSpawnedServerExits covers the other
// half of waiting longer for a server this client spawned: a server that
// dies while starting must fail the wait at once, with its own explanation,
// instead of leaving the client to wait out the budget.
func TestWaitForServerReady_FailsFastWhenSpawnedServerExits(t *testing.T) {
	t.Parallel()

	// Nothing listens on a port that was just released.
	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	hostURL := &url.URL{Scheme: "tcp", Host: ln.Addr().String()}
	require.NoError(t, ln.Close())

	child := exitedServer(t, errors.New("exit status 1"),
		"          \n   ERROR  \n          \n  Failed to load configuration: boom.  \n\n")

	start := time.Now()
	err = waitForServerReady(t.Context(), hostURL, child)
	require.Less(t, time.Since(start), 5*time.Second, "the wait must end when the server exits")
	require.ErrorContains(t, err, "crush server exited before it was ready: exit status 1")
	require.ErrorContains(t, err, child.stderrPath)
	require.ErrorContains(t, err, "Failed to load configuration: boom.")
}

// TestWaitForServerReady_SpawnedServerLostRace covers a spawned server that
// exits because another server took the address first: the client must use
// the server that answers rather than report the exit.
func TestWaitForServerReady_SpawnedServerLostRace(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)

	child := exitedServer(t, errors.New("exit status 1"), "address already in use\n")
	require.NoError(t, waitForServerReady(t.Context(), &url.URL{Scheme: "tcp", Host: u.Host}, child))
}

// TestServerReadyTimeout pins the readiness budgets: a server this client
// spawned gets far longer than one it found, and an explicit
// CRUSH_SERVER_READY_TIMEOUT is the total budget for both.
func TestServerReadyTimeout(t *testing.T) {
	t.Setenv("CRUSH_SERVER_READY_TIMEOUT", "")
	require.Equal(t, 10*time.Second, serverReadyTimeout(false))
	require.Equal(t, 2*time.Minute, serverReadyTimeout(true))

	t.Setenv("CRUSH_SERVER_READY_TIMEOUT", "3s")
	require.Equal(t, 3*time.Second, serverReadyTimeout(false))
	require.Equal(t, 3*time.Second, serverReadyTimeout(true))

	t.Setenv("CRUSH_SERVER_READY_TIMEOUT", "bogus")
	require.Equal(t, 10*time.Second, serverReadyTimeout(false))
	require.Equal(t, 2*time.Minute, serverReadyTimeout(true))
}
