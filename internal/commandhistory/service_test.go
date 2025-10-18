package commandhistory

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

func setupTestService(t *testing.T) (context.Context, *service, *db.Queries) {
	t.Helper()

	ctx := context.Background()
	conn, err := db.Connect(ctx, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})

	queries := db.New(conn)
	svc := NewService(queries, conn)
	return ctx, svc.(*service), queries
}

func TestListBySessionReturnsChronologicalHistory(t *testing.T) {
	ctx, svc, queries := setupTestService(t)

	sessionSvc := session.NewService(queries)
	sess, err := sessionSvc.Create(ctx, "test session")
	require.NoError(t, err)

	// Seed deterministic history with known timestamps.
	rows := []struct {
		id      string
		command string
		ts      int64
	}{
		{id: "cmd-1", command: "first", ts: 1},
		{id: "cmd-2", command: "second", ts: 2},
		{id: "cmd-3", command: "third", ts: 3},
	}

	for _, row := range rows {
		_, err := svc.db.ExecContext(ctx, `
			INSERT INTO command_history (id, session_id, command, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)`,
			row.id, sess.ID, row.command, row.ts, row.ts,
		)
		require.NoError(t, err)
	}

	history, err := svc.ListBySession(ctx, sess.ID, 0)
	require.NoError(t, err)
	require.Len(t, history, 3)
	require.Equal(t, []string{"first", "second", "third"}, []string{
		history[0].Command,
		history[1].Command,
		history[2].Command,
	})

	limitedHistory, err := svc.ListBySession(ctx, sess.ID, 2)
	require.NoError(t, err)
	require.Len(t, limitedHistory, 2)
	require.Equal(t, []string{"second", "third"}, []string{
		limitedHistory[0].Command,
		limitedHistory[1].Command,
	})
}
