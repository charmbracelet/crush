package backend

import (
	"context"
	"testing"

	"github.com/asx8678/ultra/internal/app"
	"github.com/asx8678/ultra/internal/db"
	"github.com/asx8678/ultra/internal/message"
	"github.com/asx8678/ultra/internal/proto"
	"github.com/asx8678/ultra/internal/session"
	"github.com/stretchr/testify/require"
	"uuid"
)

func TestRunShellCommand_SkipsPersistenceForMissingSession(t *testing.T) {
	t.Parallel()

	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	q := db.New(conn)
	sessions := session.NewService(q, conn)
	messages := message.NewService(q)

	b, _ := newTestBackend(t)
	ws := &Workspace{
		ID:           uuid.New().String(),
		Path:         t.TempDir(),
		resolvedPath: t.TempDir(),
		clients:      make(map[string]*clientState),
		shutdownFn:   func() {},
	}
	ws.App = &app.App{
		Sessions: sessions,
		Messages: messages,
	}
	ws.ctx, ws.cancel = context.WithCancel(b.ctx)
	InsertWorkspaceForTest(b, ws)

	missingSessionID := uuid.New().String()
	resp, err := b.RunShellCommand(t.Context(), ws.ID, proto.ShellCommandRequest{
		SessionID: missingSessionID,
		Command:   "echo hello",
	})
	require.NoError(t, err)
	require.Equal(t, "hello\n", resp.Output)
	require.Zero(t, resp.ExitCode)

	stored, err := messages.List(t.Context(), missingSessionID)
	require.NoError(t, err)
	require.Empty(t, stored)
}
