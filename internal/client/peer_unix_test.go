//go:build !windows

package client

import (
	"net"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// serveLocal serves h on a unix socket owned by the current user.
func serveLocal(t *testing.T, h http.Handler) (network, addr string) {
	t.Helper()
	// macOS caps socket paths at 104 bytes and t.TempDir embeds the test
	// name, so both that name and the socket's must stay short.
	path := filepath.Join(t.TempDir(), "s.sock")
	ln, err := net.Listen("unix", path) //nolint:noctx
	require.NoError(t, err)
	srv := &http.Server{Handler: h}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return "unix", path
}

// pretendOtherUser makes the client expect a server run by a uid other
// than ours, as if our own sockets belonged to another user.
func pretendOtherUser(t *testing.T) {
	t.Helper()
	realEUID := geteuid
	geteuid = func() int { return realEUID() + 1 }
	t.Cleanup(func() { geteuid = realEUID })
}
