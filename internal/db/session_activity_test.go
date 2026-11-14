package db

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSessionActivityFieldsAdvanceTimestamp(t *testing.T) {
	for _, tt := range []struct {
		name   string
		column string
		before any
		after  any
	}{
		{name: "messages", column: "message_count", before: 0, after: 1},
		{name: "prompt", column: "prompt_tokens", before: 0, after: 1},
		{name: "completion", column: "completion_tokens", before: 0, after: 1},
		{name: "cost", column: "cost", before: 0, after: 0.25},
		{name: "summary/set", column: "summary_message_id", after: "summary"},
		{name: "summary/clear", column: "summary_message_id", before: "summary"},
		{name: "todos/set", column: "todos", after: "[]"},
		{name: "todos/clear", column: "todos", before: "[]"},
		{name: "channel/set", column: "channel", after: "signal"},
		{name: "channel/clear", column: "channel", before: "signal"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dataDir := t.TempDir()
			t.Cleanup(func() { require.NoError(t, Release(dataDir)) })
			conn, err := Connect(t.Context(), dataDir)
			require.NoError(t, err)

			// Update one field at a time so a missing trigger condition is visible.
			_, err = conn.ExecContext(t.Context(), `
				INSERT INTO sessions (id, title, created_at, updated_at, `+tt.column+`)
				VALUES ('active', 'Conversation', 1, 100, ?)
			`, tt.before)
			require.NoError(t, err)
			_, err = conn.ExecContext(t.Context(), "UPDATE sessions SET "+tt.column+" = ? WHERE id = 'active'", tt.after)
			require.NoError(t, err)

			updated, err := New(conn).GetSessionByID(t.Context(), "active")
			require.NoError(t, err)
			require.Greater(t, updated.UpdatedAt, int64(100))
		})
	}
}
