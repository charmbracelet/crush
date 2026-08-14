package workspace

import (
	"testing"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/stretchr/testify/require"
)

// TestAppWorkspace_QueueOpsWithUninitializedAgent pins the local half of the
// queue contract for a workspace whose coder agent was never initialized.
// Popping used to return ErrAgentNotInitialized here while the remote path
// answered "empty queue" (Backend.PopQueuedMessage, covered by
// TestPopQueuedMessageEmptyAndErrors), so the same key press showed an error
// banner locally and did nothing in client/server mode. No coordinator means
// no queue, so every queue operation collapses to zero values.
func TestAppWorkspace_QueueOpsWithUninitializedAgent(t *testing.T) {
	t.Parallel()

	ws := NewAppWorkspace(&app.App{}, nil)
	require.Nil(t, ws.app.AgentCoordinator, "test premise: agent not initialized")

	popped, found, err := ws.AgentPopQueuedMessage("session")
	require.NoError(t, err)
	require.False(t, found)
	require.Equal(t, agent.QueuedMessage{}, popped)

	drained, err := ws.AgentClearQueue("session")
	require.NoError(t, err)
	require.Nil(t, drained)

	require.Zero(t, ws.AgentQueuedPrompts("session"))
	require.Empty(t, ws.AgentQueuedPromptsList("session"))

	// The uninitialized agent is still reported where the UI acts on it.
	require.ErrorIs(t, ws.AgentReadyErr(), ErrAgentNotInitialized)
	require.False(t, ws.AgentIsReady())
}
