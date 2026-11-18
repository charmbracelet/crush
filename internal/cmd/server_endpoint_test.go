package cmd

import (
	"context"
	"errors"
	"io/fs"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestStatServerEndpoint_NamedPipe guards stat'ing a named pipe: a busy pipe
// was taken for a stale socket, which aborted startup. Only "not found" may
// report that the pipe does not exist.
func TestStatServerEndpoint_NamedPipe(t *testing.T) {
	// Not parallel: it swaps probePipe.
	hostURL := &url.URL{Scheme: "npipe", Host: "//./pipe/crush-test.sock"}
	orig := probePipe
	t.Cleanup(func() { probePipe = orig })

	for _, tc := range []struct {
		dial   error
		exists bool
	}{
		{dial: nil, exists: true},
		{dial: errors.New("all pipe instances are busy"), exists: true},
		{dial: context.DeadlineExceeded, exists: true},
		{dial: fs.ErrPermission, exists: true},
		{dial: &fs.PathError{Op: "open", Path: hostURL.Host, Err: fs.ErrNotExist}},
	} {
		calls := 0
		probePipe = func(_ context.Context, addr string) error {
			require.Equal(t, hostURL.Host, addr)
			calls++
			return tc.dial
		}
		err := statServerEndpoint(t.Context(), hostURL)
		if tc.exists {
			require.NoError(t, err, "dial error %v", tc.dial)
		} else {
			require.ErrorIs(t, err, fs.ErrNotExist)
		}
		require.Equal(t, 1, calls, "a pipe must be dialed, not stat'ed")
	}
}
