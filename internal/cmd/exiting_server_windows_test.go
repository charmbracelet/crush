//go:build windows

package cmd

import (
	"testing"

	"github.com/Microsoft/go-winio"
	"github.com/charmbracelet/crush/internal/server"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestAwaitExitingServerGone_WaitsForPipeRelease is the named pipe
// counterpart of TestAwaitExitingServerGone_WaitsForRelease. Windows
// clients use a pipe by default, and a pipe that is gone reports that it
// does not exist rather than refusing the connection.
func TestAwaitExitingServerGone_WaitsForPipeRelease(t *testing.T) {
	t.Parallel()

	hostURL, err := server.ParseHostURL("npipe:////./pipe/crush-test-" + uuid.NewString())
	require.NoError(t, err)
	ln, err := winio.ListenPipe(hostURL.Host, &winio.PipeConfig{
		MessageMode:      true,
		InputBufferSize:  65536,
		OutputBufferSize: 65536,
	})
	require.NoError(t, err)
	srv := server.NewServer(nil, hostURL.Scheme, hostURL.Host)
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	requireWaitsForRelease(t, srv, hostURL)
}
