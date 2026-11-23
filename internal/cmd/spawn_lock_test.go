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

func commandWithContext(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetContext(ctx)
	return cmd
}

// TestSpawnAndWaitReady_LockWait guards waiting on another client's
// start.lock: Ctrl+C used to spawn a server anyway, and a suspended lock
// holder made every other client wait forever.
func TestSpawnAndWaitReady_LockWait(t *testing.T) {
	// Not parallel: it swaps startDetachedServer and sets env vars.
	for _, tc := range []struct {
		name       string
		cancel     bool
		wantSpawns int32
	}{
		{name: "cancel does not spawn", cancel: true},
		{name: "stuck lock holder", wantSpawns: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CRUSH_CACHE_DIR", t.TempDir())
			t.Setenv("CRUSH_SERVER_READY_TIMEOUT", "500ms")
			srv := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
			t.Cleanup(srv.Close)
			hostURL := &url.URL{Scheme: "tcp", Host: srv.Listener.Addr().String()}

			dir, err := perHostServerDir(hostURL)
			require.NoError(t, err)
			release, err := lock.File(t.Context(), filepath.Join(dir, "start.lock"))
			require.NoError(t, err)
			t.Cleanup(release)

			var spawns atomic.Int32
			orig := startDetachedServer
			t.Cleanup(func() { startDetachedServer = orig })
			startDetachedServer = func(*cobra.Command, *url.URL) (*spawnedServer, error) {
				if spawns.Add(1) == 1 {
					srv.Start()
				}
				return nil, nil
			}

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			if tc.cancel {
				time.AfterFunc(100*time.Millisecond, cancel)
			}
			err = spawnAndWaitReady(commandWithContext(ctx), hostURL)
			if tc.cancel {
				require.ErrorIs(t, err, context.Canceled)
			} else {
				require.NoError(t, ctx.Err(), "still waiting for the stuck lock holder")
				require.NoError(t, err)
			}
			require.Equal(t, tc.wantSpawns, spawns.Load())
		})
	}
}
