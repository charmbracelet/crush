package client

import (
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/charmbracelet/crush/internal/proto"
	"github.com/stretchr/testify/require"
)

// TestDialRefusesOtherUser guards that the client sends nothing, least of
// all its environment, to a server socket or pipe owned by another user.
func TestDialRefusesOtherUser(t *testing.T) {
	// Not parallel: pretendOtherUser swaps a package-level hook.
	pretendOtherUser(t)

	var requests atomic.Int32
	network, addr := serveLocal(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))

	c, err := NewClient(t.TempDir(), network, addr)
	require.NoError(t, err)
	_, err = c.CreateWorkspace(t.Context(), proto.Workspace{Path: c.Path(), Env: []string{"SECRET=x"}})
	require.ErrorContains(t, err, "owned by another user")
	require.Zero(t, requests.Load())
}
