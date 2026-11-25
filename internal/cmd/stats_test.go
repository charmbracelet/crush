package cmd

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/stretchr/testify/require"
)

func TestMergeStatsAverageResponseTime(t *testing.T) {
	t.Parallel()

	// newProject creates a project database with the given number of
	// assistant responses, each taking seconds, plus a number of user
	// messages that have no response time.
	newProject := func(responses, seconds, others int) ProjectStats {
		conn, err := db.Connect(t.Context(), t.TempDir())
		require.NoError(t, err)
		t.Cleanup(func() { _ = conn.Close() })

		exec := func(query string, args ...any) {
			_, err := conn.ExecContext(t.Context(), query, args...)
			require.NoError(t, err)
		}
		exec(`INSERT INTO sessions (id, title, updated_at, created_at) VALUES ('s', 't', 1000, 1000)`)
		insert := func(id, role string, finishedAt sql.NullInt64) {
			exec(`INSERT INTO messages (id, session_id, role, created_at, updated_at, finished_at)
				VALUES (?, 's', ?, 1000, 1000, ?)`, id, role, finishedAt)
		}
		for i := range responses {
			insert(fmt.Sprintf("a%d", i), "assistant", sql.NullInt64{Int64: int64(1000 + seconds), Valid: true})
		}
		for i := range others {
			insert(fmt.Sprintf("u%d", i), "user", sql.NullInt64{})
		}

		stats, err := gatherStats(t.Context(), conn)
		require.NoError(t, err)
		return ProjectStats{Stats: stats}
	}

	merged := mergeStats([]ProjectStats{
		newProject(1, 10, 99),
		newProject(9, 2, 1),
	})

	// (1*10s + 9*2s) / 10 responses.
	require.InDelta(t, 2800, merged.AvgResponseTimeMs, 0.001)
}
