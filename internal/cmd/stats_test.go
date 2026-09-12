package cmd

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

func TestGatherPruningStats(t *testing.T) {
	t.Parallel()

	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	queries := db.New(conn)
	sessions := session.NewService(queries, conn)
	messages := message.NewService(queries)

	sess, err := sessions.Create(t.Context(), "test")
	require.NoError(t, err)

	big := strings.Repeat("x", 1000)
	mk := func(role message.MessageRole, parts ...message.ContentPart) {
		t.Helper()
		_, err := messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
			Role:  role,
			Parts: parts,
		})
		require.NoError(t, err)
	}

	mk(message.Assistant, message.ToolCall{ID: "tc-1", Name: "view", Finished: true})
	mk(message.Tool, message.ToolResult{
		ToolCallID: "tc-1", Name: "view", Content: big,
		Superseded: &message.SupersededMark{Path: "a.go", ByTool: "edit", Turn: 1, Applied: true},
	})
	mk(message.Tool, message.ToolResult{
		ToolCallID: "tc-2", Name: "bash", Content: big,
		Superseded: &message.SupersededMark{Turn: 2, Applied: true, Kind: message.StubKindStale},
	})
	// Pending marks are not counted — nothing rendered yet.
	mk(message.Tool, message.ToolResult{
		ToolCallID: "tc-3", Name: "view", Content: big,
		Superseded: &message.SupersededMark{Path: "b.go", ByTool: "edit", Turn: 3},
	})
	// Plain results are not counted.
	mk(message.Tool, message.ToolResult{ToolCallID: "tc-4", Name: "view", Content: big})

	stats, err := gatherPruningStats(t.Context(), queries)
	require.NoError(t, err)
	require.NotNil(t, stats)
	require.EqualValues(t, 2, stats.StubbedResults)
	require.EqualValues(t, 1, stats.Sessions)
	require.Positive(t, stats.SavedBytes)
	require.Less(t, stats.SavedBytes, int64(2*len(big)))

	kinds := make(map[string]PruningKindStats)
	for _, k := range stats.ByKind {
		kinds[k.Kind] = k
	}
	require.EqualValues(t, 1, kinds["superseded"].Results)
	require.EqualValues(t, 1, kinds["stale"].Results)
}

func TestGatherPruningStats_Empty(t *testing.T) {
	t.Parallel()

	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	stats, err := gatherPruningStats(t.Context(), db.New(conn))
	require.NoError(t, err)
	require.Nil(t, stats)
}
