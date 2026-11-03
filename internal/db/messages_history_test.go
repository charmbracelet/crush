package db

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestListAllUserMessages_ExcludesChildSessions pins that prompt history only
// holds what the user typed: a subagent's task prompt is a user message in a
// child session, and must not show up when no session is open.
func TestListAllUserMessages_ExcludesChildSessions(t *testing.T) {
	t.Parallel()

	conn, err := Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	q := New(conn)

	_, err = q.CreateSession(t.Context(), CreateSessionParams{ID: "root", Title: "root"})
	require.NoError(t, err)
	_, err = q.CreateSession(t.Context(), CreateSessionParams{
		ID:              "child",
		ParentSessionID: sql.NullString{String: "root", Valid: true},
		Title:           "child",
	})
	require.NoError(t, err)

	for _, m := range []CreateMessageParams{
		{ID: "typed", SessionID: "root", Role: "user", Parts: "[]"},
		{ID: "task-prompt", SessionID: "child", Role: "user", Parts: "[]"},
		{ID: "reply", SessionID: "root", Role: "assistant", Parts: "[]"},
	} {
		_, err = q.CreateMessage(t.Context(), m)
		require.NoError(t, err)
	}

	got, err := q.ListAllUserMessages(t.Context())
	require.NoError(t, err)

	ids := make([]string, 0, len(got))
	for _, m := range got {
		ids = append(ids, m.ID)
	}
	require.Equal(t, []string{"typed"}, ids)
}

// TestListAllUserMessages_KeepsRoleIndexOrdering guards the session join: the
// real query must still seek the role index and get its ordering from it,
// not scan the table or sort the result.
func TestListAllUserMessages_KeepsRoleIndexOrdering(t *testing.T) {
	t.Parallel()

	conn, err := Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	rows, err := conn.QueryContext(t.Context(), "EXPLAIN QUERY PLAN "+listAllUserMessages)
	require.NoError(t, err)
	defer rows.Close()

	var plan strings.Builder
	for rows.Next() {
		var id, parent, notused int
		var detail string
		require.NoError(t, rows.Scan(&id, &parent, &notused, &detail))
		plan.WriteString(detail)
		plan.WriteString("\n")
	}
	require.NoError(t, rows.Err())

	require.Contains(t, plan.String(), "idx_messages_role_created_at")
	require.NotContains(t, plan.String(), "TEMP B-TREE")
}
