package backend

import (
	"context"
	"testing"
	"uuid"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

// insertMessageWorkspace installs a workspace backed by real session and
// message services and the given coordinator, which may be nil.
func insertMessageWorkspace(t *testing.T, b *Backend, coord agent.Coordinator) (*Workspace, session.Service, message.Service) {
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
	ws.App = &app.App{Sessions: sessions, Messages: messages, AgentCoordinator: coord}
	ws.ctx, ws.cancel = context.WithCancel(b.ctx)
	b.mu.Lock()
	b.workspaces.Set(ws.ID, ws)
	b.pathIndex[ws.resolvedPath] = ws.ID
	b.mu.Unlock()
	return ws, sessions, messages
}

// settlingCoordinator records the session it was asked to read.
type settlingCoordinator struct {
	agent.Coordinator
	read []string
	msgs []message.Message
}

func (c *settlingCoordinator) ListSettledMessages(_ context.Context, sessionID string) ([]message.Message, error) {
	c.read = append(c.read, sessionID)
	return c.msgs, nil
}

// Every remote client reads its transcript through the backend, so the read
// must go through the agent, which owns settling interrupted calls.
func TestListSessionMessages_ReadsThroughAgent(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	coord := &settlingCoordinator{msgs: []message.Message{{ID: "settled"}}}
	ws, _, _ := insertMessageWorkspace(t, b, coord)

	msgs, err := b.ListSessionMessages(t.Context(), ws.ID, "sess")
	require.NoError(t, err)
	require.Equal(t, []string{"sess"}, coord.read)
	require.Equal(t, coord.msgs, msgs)
}

// With no agent attached the stored transcript is returned as is.
func TestListSessionMessages_WithoutAgent(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	ws, sessions, messages := insertMessageWorkspace(t, b, nil)

	sess, err := sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	_, err = messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "hi"}},
	})
	require.NoError(t, err)

	msgs, err := b.ListSessionMessages(t.Context(), ws.ID, sess.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
}
