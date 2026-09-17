package server

import (
	"sync/atomic"
	"testing"

	"github.com/charmbracelet/crush/internal/backend"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestShutdown_TearsDownHostedWorkspaces covers the path a termination
// signal takes: stopping the server must tear down live workspaces so
// their interrupted work is recorded.
func TestShutdown_TearsDownHostedWorkspaces(t *testing.T) {
	t.Parallel()

	srv := NewServer(nil, "tcp", "127.0.0.1:0")
	var tornDown atomic.Int32
	ws := &backend.Workspace{ID: uuid.New().String(), Path: t.TempDir()}
	backend.SetWorkspaceShutdownFnForTest(ws, func() { tornDown.Add(1) })
	backend.InsertWorkspaceForTest(srv.backend, ws)

	require.NoError(t, srv.Shutdown(t.Context()))
	require.Equal(t, int32(1), tornDown.Load())
}
