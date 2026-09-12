package db

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestUserMessagesUseRoleIndex pins why idx_messages_role_created_at exists:
// without it, listing user messages scans the whole table.
func TestUserMessagesUseRoleIndex(t *testing.T) {
	t.Parallel()

	conn, err := Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	var plan strings.Builder
	rows, err := conn.QueryContext(t.Context(),
		`EXPLAIN QUERY PLAN SELECT * FROM messages WHERE role = 'user' ORDER BY created_at DESC`)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var id, parent, notused int
		var detail string
		require.NoError(t, rows.Scan(&id, &parent, &notused, &detail))
		plan.WriteString(detail)
		plan.WriteString("\n")
	}
	require.NoError(t, rows.Err())

	got := plan.String()
	require.Contains(t, got, "idx_messages_role_created_at",
		"listing user messages must use the role index, not walk the table")
	// role first, created_at second, so the rows arrive already ordered.
	require.NotContains(t, got, "TEMP B-TREE",
		"the index covers the ordering, so no sort should be needed")
}
