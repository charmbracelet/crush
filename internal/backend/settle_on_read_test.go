package backend

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// insertMessageWorkspace installs a workspace backed by real session and
// message services, with no agent coordinator, so nothing is ever busy.
func insertMessageWorkspace(t *testing.T, b *Backend) (*Workspace, session.Service, message.Service) {
	t.Helper()
	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	q := db.New(conn)
	sessions := session.NewService(q, conn)
	messages := message.NewService(q)

	ws := &Workspace{
		ID:           uuid.New().String(),
		Path:         t.TempDir(),
		resolvedPath: t.TempDir(),
		clients:      make(map[string]*clientState),
		shutdownFn:   func() {},
	}
	ws.App = &app.App{Sessions: sessions, Messages: messages}
	ws.ctx, ws.cancel = context.WithCancel(b.ctx)
	b.mu.Lock()
	b.workspaces.Set(ws.ID, ws)
	b.pathIndex[ws.resolvedPath] = ws.ID
	b.mu.Unlock()
	return ws, sessions, messages
}

// A tool call whose process died has a request and no reply. Every remote
// client reads its transcript through the backend, so the repair has to
// happen here too: without it a client/server session shows the call as
// still running for good, however many times it is reopened.
func TestListSessionMessages_SettlesInterruptedCalls(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	ws, sessions, messages := insertMessageWorkspace(t, b)

	sess, err := sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	_, err = messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: "run something"},
		},
	})
	require.NoError(t, err)

	// An assistant turn that asked for a tool and never came back: no
	// finish part, no tool result.
	_, err = messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{ID: "call-1", Name: "bash", Input: "{}", Finished: true},
		},
	})
	require.NoError(t, err)

	msgs, err := b.ListSessionMessages(t.Context(), ws.ID, sess.ID)
	require.NoError(t, err)

	var results []message.ToolResult
	for _, m := range msgs {
		results = append(results, m.ToolResults()...)
	}
	require.Len(t, results, 1, "the dangling call must have been answered")
	require.Equal(t, "call-1", results[0].ToolCallID)
	require.True(t, results[0].IsError, "an interrupted call settles as an error")
}

// A healthy transcript must be returned untouched: settling is a repair,
// not a rewrite.
func TestListSessionMessages_LeavesSettledTranscriptAlone(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	ws, sessions, messages := insertMessageWorkspace(t, b)

	sess, err := sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	_, err = messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "hi"}},
	})
	require.NoError(t, err)

	before, err := b.ListSessionMessages(t.Context(), ws.ID, sess.ID)
	require.NoError(t, err)
	after, err := b.ListSessionMessages(t.Context(), ws.ID, sess.ID)
	require.NoError(t, err)

	require.Len(t, after, len(before), "a second read must not add messages")
}

// The repair must never touch a session a live run still holds, since an
// unanswered call there is not an orphan yet.
func TestReadSettledMessages_SkipsBusySessions(t *testing.T) {
	t.Parallel()
	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	q := db.New(conn)
	sessions := session.NewService(q, conn)
	messages := message.NewService(q)

	sess, err := sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	_, err = messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{ID: "call-1", Name: "bash", Input: "{}", Finished: true},
		},
	})
	require.NoError(t, err)

	busy := agent.SessionBusyFunc(func(string) bool { return true })
	msgs, err := agent.ReadSettledMessages(t.Context(), messages, busy, sess.ID)
	require.NoError(t, err)

	for _, m := range msgs {
		require.Empty(t, m.ToolResults(),
			"a live run's unanswered call must be left alone")
	}
}
