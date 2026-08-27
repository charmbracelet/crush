package backend

import (
	"testing"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

// newStoreWorkspace installs a workspace backed by a real SQLite store
// and the given coordinator, which is what the repair-on-read path needs:
// message writes plus a busy verdict.
func newStoreWorkspace(t *testing.T, b *Backend, coord *blockingCoordinator) (*Workspace, session.Service, message.Service) {
	t.Helper()
	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	q := db.New(conn)
	sessions := session.NewService(q, conn)
	messages := message.NewService(q)

	ws := insertAgentWorkspace(t, b, coord)
	ws.Sessions = sessions
	ws.Messages = messages
	return ws, sessions, messages
}

// storeFrozenTurn writes the transcript a killed run leaves behind: an
// assistant message with no finish part and a tool call with no result.
func storeFrozenTurn(t *testing.T, messages message.Service, sessionID string) string {
	t.Helper()
	msg, err := messages.Create(t.Context(), sessionID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: "working"},
			message.ToolCall{ID: "call-1", Name: "bash", Input: "{}", Finished: true},
		},
	})
	require.NoError(t, err)
	return msg.ID
}

// TestListSessionMessages_RepairsIdleSession proves the wiring: reading
// an idle session's transcript gives an abandoned turn a terminal state,
// so a client that reconnects after the server died sees an interrupted
// turn rather than a spinner.
func TestListSessionMessages_RepairsIdleSession(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	ws, sessions, messages := newStoreWorkspace(t, b, newBlockingCoordinator())
	sess, err := sessions.Create(t.Context(), "frozen")
	require.NoError(t, err)
	msgID := storeFrozenTurn(t, messages, sess.ID)

	msgs, err := b.ListSessionMessages(t.Context(), ws.ID, sess.ID)
	require.NoError(t, err)

	var repaired *message.Message
	var result *message.ToolResult
	for i := range msgs {
		if msgs[i].ID == msgID {
			repaired = &msgs[i]
		}
		for _, tr := range msgs[i].ToolResults() {
			if tr.ToolCallID == "call-1" {
				result = &tr
			}
		}
	}
	require.NotNil(t, repaired)
	require.True(t, repaired.IsFinished(), "the read must leave no unfinished turn")
	require.Equal(t, message.FinishReasonError, repaired.FinishReason())
	require.NotNil(t, result, "the unanswered tool call must get a result")
	require.True(t, result.IsError)
}

// TestListSessionMessages_SkipsBusySession keeps the repair off a live
// turn. A run streams into exactly these messages, so rewriting them
// mid-flight would finish a turn that is still going.
func TestListSessionMessages_SkipsBusySession(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	coord := newBlockingCoordinator()
	ws, sessions, messages := newStoreWorkspace(t, b, coord)
	sess, err := sessions.Create(t.Context(), "live")
	require.NoError(t, err)
	msgID := storeFrozenTurn(t, messages, sess.ID)

	coord.mu.Lock()
	coord.running[sess.ID] = true
	coord.mu.Unlock()

	msgs, err := b.ListSessionMessages(t.Context(), ws.ID, sess.ID)
	require.NoError(t, err)

	for i := range msgs {
		if msgs[i].ID == msgID {
			require.False(t, msgs[i].IsFinished(),
				"a streaming turn must not be finished by a reader")
			require.Empty(t, msgs[i].ToolResults())
		}
	}
}
