//go:build windows

package client

import (
	"net/http"
	"testing"

	"github.com/Microsoft/go-winio"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

// serveLocal serves h on a named pipe created by the current user, set up
// like the server's own.
func serveLocal(t *testing.T, h http.Handler) (network, addr string) {
	t.Helper()
	addr = `\\.\pipe\crush-test-` + uuid.NewString()
	ln, err := winio.ListenPipe(addr, &winio.PipeConfig{
		MessageMode:      true,
		InputBufferSize:  65536,
		OutputBufferSize: 65536,
	})
	require.NoError(t, err)
	srv := &http.Server{Handler: h}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return "npipe", addr
}

// pretendOtherUser makes the client expect a pipe server run by a SID no
// process runs as, as if our own pipes belonged to another user.
func pretendOtherUser(t *testing.T) {
	t.Helper()
	realUser := currentUser
	currentUser = func() (*windows.SID, error) {
		return windows.CreateWellKnownSid(windows.WinWorldSid)
	}
	t.Cleanup(func() { currentUser = realUser })
}
