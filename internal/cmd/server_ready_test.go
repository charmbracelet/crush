package cmd

import (
	"errors"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestServerReadyTimeout guards a slow-starting server this client spawned
// being given up on after the short budget meant for a server it found.
func TestServerReadyTimeout(t *testing.T) {
	t.Setenv("CRUSH_SERVER_READY_TIMEOUT", "")
	require.Equal(t, 10*time.Second, serverReadyTimeout(false))
	require.Equal(t, 2*time.Minute, serverReadyTimeout(true))
	t.Setenv("CRUSH_SERVER_READY_TIMEOUT", "3s")
	require.Equal(t, 3*time.Second, serverReadyTimeout(true))
}

// TestWaitForServerReady_FailsFastWhenSpawnedServerExits checks that the
// longer budget is cut short, with the server's own error, when it dies.
func TestWaitForServerReady_FailsFastWhenSpawnedServerExits(t *testing.T) {
	t.Parallel()

	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	hostURL := &url.URL{Scheme: "tcp", Host: ln.Addr().String()}
	require.NoError(t, ln.Close())

	stderr := filepath.Join(t.TempDir(), "stderr.log")
	require.NoError(t, os.WriteFile(stderr, []byte("  Failed to load configuration: boom.\n"), 0o600))
	child := &spawnedServer{stderrPath: stderr, done: make(chan struct{}), err: errors.New("exit status 1")}
	close(child.done)

	start := time.Now()
	err = waitForServerReady(t.Context(), hostURL, child)
	require.Less(t, time.Since(start), 5*time.Second)
	require.ErrorContains(t, err, "exited before it was ready: exit status 1")
	require.ErrorContains(t, err, "Failed to load configuration: boom.")
}
