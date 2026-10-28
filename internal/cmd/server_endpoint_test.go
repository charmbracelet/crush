package cmd

import (
	"context"
	"errors"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// errPipeBusy stands in for ERROR_PIPE_BUSY, which a single open of a pipe
// name fails with while none of its instances is listening.
var errPipeBusy = errors.New("all pipe instances are busy")

// stubPipeProbe replaces probePipe for the duration of the test with one
// that returns results in turn, repeating the last, and reports how many
// times it was called.
func stubPipeProbe(t *testing.T, addr string, results ...error) *int {
	t.Helper()
	orig := probePipe
	t.Cleanup(func() { probePipe = orig })
	calls := 0
	probePipe = func(_ context.Context, got string) error {
		require.Equal(t, addr, got)
		err := results[min(calls, len(results)-1)]
		calls++
		return err
	}
	return &calls
}

// TestStatServerEndpoint_NamedPipe covers deciding whether a server's pipe
// exists. Only "not found" may say it does not: every other outcome, busy
// above all, used to reach ensureServer's stale-socket branch, which tried
// to delete the pipe and aborted startup when that failed.
func TestStatServerEndpoint_NamedPipe(t *testing.T) {
	hostURL := &url.URL{Scheme: "npipe", Host: "//./pipe/crush-test.sock"}
	notFound := &fs.PathError{Op: "open", Path: hostURL.Host, Err: fs.ErrNotExist}

	for _, tc := range []struct {
		name   string
		dial   error
		exists bool
	}{
		{name: "listening", dial: nil, exists: true},
		{name: "busy", dial: errPipeBusy, exists: true},
		{name: "busy until the deadline", dial: context.DeadlineExceeded, exists: true},
		{name: "access denied", dial: fs.ErrPermission, exists: true},
		{name: "not found", dial: notFound, exists: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := stubPipeProbe(t, hostURL.Host, tc.dial)
			err := statServerEndpoint(t.Context(), hostURL)
			if tc.exists {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, fs.ErrNotExist)
			}
			require.Equal(t, 1, *calls, "a pipe must be dialed, not stat'ed")
		})
	}
}

// TestStatServerEndpoint_UnixStatsSocketFile pins the unix behavior: the
// socket file is stat'ed and the pipe probe is never used.
func TestStatServerEndpoint_UnixStatsSocketFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "crush.sock")
	hostURL := &url.URL{Scheme: "unix", Host: path}
	calls := stubPipeProbe(t, path, nil)

	require.ErrorIs(t, statServerEndpoint(t.Context(), hostURL), fs.ErrNotExist)
	require.NoError(t, os.WriteFile(path, nil, 0o600))
	require.NoError(t, statServerEndpoint(t.Context(), hostURL))
	require.Zero(t, *calls)
}

// TestAwaitSocketGone_NamedPipe covers waiting out an exiting server's
// pipe: it is dialed until its name is gone.
func TestAwaitSocketGone_NamedPipe(t *testing.T) {
	hostURL := &url.URL{Scheme: "npipe", Host: "//./pipe/crush-test.sock"}
	notFound := &fs.PathError{Op: "open", Path: hostURL.Host, Err: fs.ErrNotExist}
	calls := stubPipeProbe(t, hostURL.Host,
		context.DeadlineExceeded, errPipeBusy, nil, notFound)

	require.NoError(t, awaitSocketGone(t.Context(), hostURL))
	require.Equal(t, 4, *calls)
}

// TestAwaitSocketGone_KeepsNamedPipe covers a pipe that outlives the wait.
// It must not be removed: removing a pipe opens it as a client, which can
// never delete it, and the name goes away with the old server anyway.
func TestAwaitSocketGone_KeepsNamedPipe(t *testing.T) {
	// A regular file stands in for the pipe so that a removal shows.
	path := filepath.Join(t.TempDir(), "crush.sock")
	require.NoError(t, os.WriteFile(path, nil, 0o600))
	hostURL := &url.URL{Scheme: "npipe", Host: path}
	stubPipeProbe(t, path, context.DeadlineExceeded)

	require.NoError(t, awaitSocketGone(t.Context(), hostURL))
	require.FileExists(t, path)
}
