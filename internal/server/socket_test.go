//go:build !windows

package server

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// fakeTimeoutErr is a minimal net.Error implementation whose Timeout()
// returns true. It is used to verify that IsStaleSocketErr never
// classifies a timeout as stale.
type fakeTimeoutErr struct{}

func (fakeTimeoutErr) Error() string   { return "fake timeout" }
func (fakeTimeoutErr) Timeout() bool   { return true }
func (fakeTimeoutErr) Temporary() bool { return true }

func TestIsStaleSocketErr(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "ECONNREFUSED", err: syscall.ECONNREFUSED, want: true},
		{
			name: "wrapped ECONNREFUSED",
			err:  fmt.Errorf("dial: %w", syscall.ECONNREFUSED),
			want: true,
		},
		{name: "fs.ErrNotExist", err: fs.ErrNotExist, want: true},
		{
			name: "wrapped fs.ErrNotExist",
			err:  fmt.Errorf("stat: %w", fs.ErrNotExist),
			want: true,
		},
		{name: "timeout net.Error", err: fakeTimeoutErr{}, want: false},
		{name: "generic error", err: errors.New("boom"), want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, IsStaleSocketErr(tc.err))
		})
	}
}

func TestDefaultHost_XDGRuntimeDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)

	host := DefaultHost()

	require.True(t, strings.HasPrefix(host, "unix://"),
		"DefaultHost should return a unix:// URL, got %q", host)
	path := strings.TrimPrefix(host, "unix://")

	// The composed path may exceed maxUnixSocketPathLen and fall back
	// to /tmp; only assert containment when it did not.
	if len(filepath.Join(dir, "crush.sock")) <= maxUnixSocketPathLen {
		require.True(t, strings.HasPrefix(path, dir),
			"socket path %q should live under %q", path, dir)
	}
	require.True(t, strings.HasSuffix(path, ".sock"),
		"socket path %q should end in .sock", path)
	require.Contains(t, filepath.Base(path), "crush",
		"socket filename should contain 'crush'")
}

func TestDefaultHost_FallbackTemp(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")

	host := DefaultHost()

	require.True(t, strings.HasPrefix(host, "unix://"),
		"DefaultHost should return a unix:// URL, got %q", host)
	path := strings.TrimPrefix(host, "unix://")
	require.NotEmpty(t, path, "fallback socket path must be non-empty")
	require.True(t, strings.HasSuffix(path, ".sock"),
		"socket path %q should end in .sock", path)
	require.Contains(t, filepath.Base(path), "crush",
		"socket filename should contain 'crush'")
}

// staleSocketPath creates a deterministic stale unix socket file on
// disk: the socket node exists but no goroutine is accepting on it.
// It does so by binding a listener, disabling unlink-on-close, then
// closing the listener. The path is returned so the caller can probe
// it. A leftover file is best-effort removed via t.Cleanup.
func staleSocketPath(t *testing.T, path string) {
	t.Helper()
	ln, err := net.Listen("unix", path)
	require.NoError(t, err)
	ul, ok := ln.(*net.UnixListener)
	require.True(t, ok, "expected *net.UnixListener, got %T", ln)
	ul.SetUnlinkOnClose(false)
	require.NoError(t, ul.Close())

	// Verify it is actually stale: dialing should fail.
	conn, dialErr := net.DialTimeout("unix", path, 200*time.Millisecond)
	if dialErr == nil {
		conn.Close()
		t.Fatalf("expected stale socket at %q to refuse connections", path)
	}
	require.True(t, IsStaleSocketErr(dialErr),
		"expected stale-socket dial error, got %v", dialErr)

	t.Cleanup(func() {
		_ = os.Remove(path)
	})
}

func TestListen_RemovesStaleSocket(t *testing.T) {
	// t.TempDir() yields a path that may already be near the macOS
	// sun_path limit; use a short filename to stay well under it.
	dir := t.TempDir()
	path := filepath.Join(dir, "s.sock")

	staleSocketPath(t, path)

	// Confirm the stale node is present before we call listen.
	_, statErr := os.Stat(path)
	require.NoError(t, statErr, "stale socket file should exist on disk")

	ln, removedStale, err := listen("unix", path)
	require.NoError(t, err)
	require.NotNil(t, ln)
	require.True(t, removedStale, "listen should report removedStale=true")
	t.Cleanup(func() {
		_ = ln.Close()
	})
}

func TestListen_LiveSocketNotRemoved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "s.sock")

	ln1, err := net.Listen("unix", path)
	require.NoError(t, err)

	// Drain accepts so the listener stays alive and responsive without
	// blocking the test on a stray connection.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			c, err := ln1.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()
	t.Cleanup(func() {
		_ = ln1.Close()
		wg.Wait()
	})

	ln2, removedStale, err := listen("unix", path)
	if ln2 != nil {
		_ = ln2.Close()
	}
	require.Error(t, err, "listen on a live socket must fail")
	require.False(t, removedStale,
		"a live socket must never be removed (got removedStale=true)")

	// The live socket file must still be on disk and dialable.
	_, statErr := os.Stat(path)
	require.NoError(t, statErr, "live socket file should still exist")
	conn, dialErr := net.DialTimeout("unix", path, 200*time.Millisecond)
	require.NoError(t, dialErr, "live socket should still accept dials")
	_ = conn.Close()
}
