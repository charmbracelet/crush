package client

import (
	"encoding/json"
	"net/http"
	"sync"
	"testing"

	"github.com/charmbracelet/crush/internal/proto"
	"github.com/stretchr/testify/require"
)

// The server listens at a predictable address that another local user can
// claim first, so the client must not send anything, least of all its
// environment, to a server that does not run as the current user.
func TestDialRefusesOtherUser(t *testing.T) {
	// Not parallel: pretendOtherUser swaps a package-level hook.
	pretendOtherUser(t)

	var (
		mu       sync.Mutex
		received []string
	)
	network, addr := serveLocal(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ws proto.Workspace
		_ = json.NewDecoder(r.Body).Decode(&ws)
		mu.Lock()
		received = append(received, r.Method+" "+r.URL.Path)
		received = append(received, ws.Env...)
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(ws)
	}))

	c, err := NewClient(t.TempDir(), network, addr)
	require.NoError(t, err)
	_, err = c.CreateWorkspace(t.Context(), proto.Workspace{
		Path: c.Path(),
		Env:  []string{"ANTHROPIC_API_KEY=sk-ant-canary"},
	})

	mu.Lock()
	defer mu.Unlock()
	require.Empty(t, received, "a server run by another user received requests")
	require.ErrorContains(t, err, "owned by another user")
}

func TestDialAcceptsCurrentUser(t *testing.T) {
	t.Parallel()

	network, addr := serveLocal(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	c, err := NewClient(t.TempDir(), network, addr)
	require.NoError(t, err)
	require.NoError(t, c.Health(t.Context()))
}
