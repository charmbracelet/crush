//go:build windows

package cmd

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/version"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// testPipeHost returns a named pipe host in the form DefaultHost produces,
// unique to the test so concurrent test processes never share a pipe.
func testPipeHost() *url.URL {
	return &url.URL{Scheme: "npipe", Host: "//./pipe/crush-test-" + rand.Text() + ".sock"}
}

// listenTestPipe binds addr the way the server does. Until Accept is called
// the pipe has no instance a client can connect to, so every open of the
// name fails with ERROR_PIPE_BUSY: the state a live server's pipe is in
// between two accepts, or while an exiting server drains its connections.
func listenTestPipe(t *testing.T, addr string) net.Listener {
	t.Helper()
	ln, err := winio.ListenPipe(addr, &winio.PipeConfig{
		MessageMode:      true,
		InputBufferSize:  65536,
		OutputBufferSize: 65536,
	})
	require.NoError(t, err)
	t.Cleanup(func() { ln.Close() })
	return ln
}

// TestEnsureServer_BusyNamedPipe is the regression test for treating the
// pipe like a socket file. os.Stat opens a pipe as a client, so with no
// instance listening it failed with ERROR_PIPE_BUSY; ensureServer took that
// for a broken socket file, tried to delete the pipe and aborted startup
// with "failed to remove stale server socket". A busy pipe belongs to a
// live server, which must be used once it accepts again.
func TestEnsureServer_BusyNamedPipe(t *testing.T) {
	// A spawned server would be a detached copy of this test binary. Make
	// the per-host server directory impossible to create, so that anything
	// that gets as far as spawning fails instead.
	blocker := filepath.Join(t.TempDir(), "blocker")
	require.NoError(t, os.WriteFile(blocker, nil, 0o600))
	t.Setenv("CRUSH_CACHE_DIR", filepath.Join(blocker, "cache"))

	hostURL := testPipeHost()
	ln := listenTestPipe(t, hostURL.Host)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/version", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(proto.VersionInfo{
			Version: version.Version,
			BuildID: version.BuildID,
		})
	})
	mux.HandleFunc("GET /v1/health", func(http.ResponseWriter, *http.Request) {})
	srv := &http.Server{Handler: mux}
	served := make(chan struct{})
	go func() {
		defer close(served)
		// Only start accepting once ensureServer has looked at the pipe.
		time.Sleep(time.Second)
		_ = srv.Serve(ln)
	}()
	t.Cleanup(func() {
		_ = srv.Close()
		<-served
	})

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	require.NoError(t, ensureServer(cmd, hostURL))
}

// TestStatServerEndpoint_RealNamedPipe checks the dial probe against the
// states a real pipe goes through.
func TestStatServerEndpoint_RealNamedPipe(t *testing.T) {
	hostURL := testPipeHost()
	require.ErrorIs(t, statServerEndpoint(t.Context(), hostURL), fs.ErrNotExist,
		"a pipe nobody created does not exist")

	ln := listenTestPipe(t, hostURL.Host)
	require.NoError(t, statServerEndpoint(t.Context(), hostURL),
		"a pipe with no instance listening still exists")

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	require.NoError(t, statServerEndpoint(t.Context(), hostURL),
		"a listening pipe exists")

	require.NoError(t, ln.Close())
	require.Eventually(t, func() bool {
		return errors.Is(statServerEndpoint(t.Context(), hostURL), fs.ErrNotExist)
	}, 5*time.Second, 50*time.Millisecond, "a closed pipe's name goes away")
}
