package cmd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/lock"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// spawnLockFixture stands in for another client that is mid-spawn: it
// holds start.lock for the whole test and replaces startDetachedServer with
// a stub that brings up a healthy server instead of a crush binary.
type spawnLockFixture struct {
	hostURL *url.URL
	spawns  atomic.Int32
}

func newSpawnLockFixture(t *testing.T) *spawnLockFixture {
	t.Helper()
	t.Setenv("CRUSH_CACHE_DIR", t.TempDir())

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	f := &spawnLockFixture{
		hostURL: &url.URL{Scheme: "tcp", Host: srv.Listener.Addr().String()},
	}

	dir, err := perHostServerDir(f.hostURL)
	require.NoError(t, err)
	release, err := lock.File(t.Context(), filepath.Join(dir, "start.lock"))
	require.NoError(t, err)
	t.Cleanup(release)

	orig := startDetachedServer
	startDetachedServer = func(*cobra.Command, *url.URL) (*spawnedServer, error) {
		if f.spawns.Add(1) == 1 {
			srv.Start()
		}
		return nil, nil
	}
	t.Cleanup(func() { startDetachedServer = orig })
	return f
}

func commandWithContext(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetContext(ctx)
	return cmd
}

// TestSpawnAndWaitReady_CancelDoesNotSpawn covers Ctrl+C while another
// client holds start.lock. The cancelled wait used to be treated like an
// unavailable lock, so the client spawned a server anyway: one the user had
// just aborted, racing the lock holder's own spawn.
func TestSpawnAndWaitReady_CancelDoesNotSpawn(t *testing.T) {
	f := newSpawnLockFixture(t)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	time.AfterFunc(100*time.Millisecond, cancel)
	err := spawnAndWaitReady(commandWithContext(ctx), f.hostURL)
	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, f.spawns.Load(), "a cancelled client must not spawn a server")
}

// TestSpawnAndWaitReady_StuckLockHolderDoesNotHang covers a client that is
// suspended (Ctrl+Z, SIGSTOP, a debugger) while it holds start.lock. Every
// other client used to wait for it forever; the wait is now bounded and the
// client falls back to spawning a server itself.
func TestSpawnAndWaitReady_StuckLockHolderDoesNotHang(t *testing.T) {
	t.Setenv("CRUSH_SERVER_READY_TIMEOUT", "500ms")
	f := newSpawnLockFixture(t)

	// Stands in for the user giving up, so an unbounded wait fails the
	// test instead of hanging it.
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	err := spawnAndWaitReady(commandWithContext(ctx), f.hostURL)
	require.NoError(t, ctx.Err(), "still waiting for the stuck lock holder")
	require.NoError(t, err)
	require.Equal(t, int32(1), f.spawns.Load(), "the client must spawn a server itself")
}
