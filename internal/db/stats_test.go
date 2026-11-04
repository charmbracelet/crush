package db

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestGetToolUsageSplitByCompression verifies the substr magic filter
// that splits tool-usage aggregation between SQL (legacy plain-JSON
// rows) and Go (zstd-compressed rows). The zstd payload is an opaque
// frame; only the leading magic number matters here.
func TestGetToolUsageSplitByCompression(t *testing.T) {
	conn, err := Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	q := New(conn)

	_, err = conn.ExecContext(t.Context(),
		"INSERT INTO sessions (id, title, message_count, updated_at, created_at) VALUES ('s1', 's', 0, 0, 0)")
	require.NoError(t, err)

	insert := `INSERT INTO messages (id, session_id, role, parts, model, provider, is_summary_message, created_at, updated_at)
		VALUES (?, 's1', 'assistant', ?, NULL, NULL, 0, 0, 0)`
	// Legacy rows, exactly as pre-compression versions wrote them.
	_, err = conn.ExecContext(t.Context(), insert, "legacy-1",
		`[{"type":"tool_call","data":{"name":"bash"}},{"type":"tool_call","data":{"name":"bash"}}]`)
	require.NoError(t, err)
	_, err = conn.ExecContext(t.Context(), insert, "legacy-2",
		`[{"type":"tool_call","data":{"name":"view"}}]`)
	require.NoError(t, err)
	// Compressed rows: leading zstd magic, rest opaque.
	_, err = conn.ExecContext(t.Context(), insert, "zstd-1",
		[]byte{0x28, 0xb5, 0x2f, 0xfd, 0x00, 0x00})
	require.NoError(t, err)
	_, err = conn.ExecContext(t.Context(), insert, "zstd-2",
		[]byte{0x28, 0xb5, 0x2f, 0xfd, 0x00, 0x00})
	require.NoError(t, err)

	usage, err := q.GetToolUsage(t.Context())
	require.NoError(t, err)
	counts := map[string]int64{}
	for _, row := range usage {
		name, ok := row.ToolName.(string)
		require.True(t, ok)
		counts[name] = row.CallCount
	}
	require.Equal(t, map[string]int64{"bash": 2, "view": 1}, counts,
		"json_each aggregation must see legacy rows and skip compressed ones")

	compressed, err := q.GetCompressedParts(t.Context())
	require.NoError(t, err)
	require.Len(t, compressed, 2, "only the two zstd rows are selected")
	for _, blob := range compressed {
		require.Equal(t, []byte{0x28, 0xb5, 0x2f, 0xfd}, blob[:4])
	}
}
