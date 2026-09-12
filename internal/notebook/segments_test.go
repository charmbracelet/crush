package notebook

import (
	"context"
	"sync"
	"testing"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// newSegmentTestService creates a notebook service with a real DB
// handle so GenerateSegmentEntries exercises the transactional path.
func newSegmentTestService(t *testing.T, gen Generator) (Service, *db.Queries, string) {
	t.Helper()
	dataDir := t.TempDir()
	conn, err := db.Connect(context.Background(), dataDir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Release(dataDir) })
	q := db.New(conn)
	sessionID := uuid.New().String()
	_, err = q.CreateSession(context.Background(), db.CreateSessionParams{
		ID:    sessionID,
		Title: "test",
	})
	require.NoError(t, err)
	if gen == nil {
		gen = &mockGenerator{entries: []GeneratedEntry{{
			EventType: EventFileEdit,
			Title:     "Edited a.go",
			Text:      "## Edited a.go\n- applied changes",
			Tags:      []string{"file:a.go"},
		}}}
	}
	svc := NewService(q, gen, Options{
		MaxEntryTokens:    1000,
		MaxNotebookTokens: 100000,
		DB:                conn,
	})
	return svc, q, sessionID
}

func segmentEditMsgs() []message.Message {
	return []message.Message{
		{Role: message.Assistant, Parts: []message.ContentPart{
			message.ToolCall{ID: "tc-e", Name: "edit", Input: `{"file_path":"a.go"}`, Finished: true},
		}},
		{Role: message.Tool, Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "tc-e", Name: "edit", Content: "ok"},
		}},
	}
}

func TestGenerateSegmentEntries_ContinuesEventNumbering(t *testing.T) {
	svc, _, sessionID := newSegmentTestService(t, nil)
	ctx := context.Background()

	require.NoError(t, svc.GenerateSegmentEntries(ctx, sessionID, 0, 0, 0, 2, segmentEditMsgs()))
	require.NoError(t, svc.GenerateSegmentEntries(ctx, sessionID, 0, 1, 2, 4, segmentEditMsgs()))

	entries, err := svc.GetEntries(ctx, sessionID)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	// Second segment continues the turn's event sequence — no
	// duplicate (turn, event) keys.
	require.Equal(t, int64(0), entries[0].SegmentNumber)
	require.Equal(t, int64(0), entries[0].EventNumber)
	require.Equal(t, int64(1), entries[1].SegmentNumber)
	require.Equal(t, int64(1), entries[1].EventNumber)
}

func TestGenerateSegmentEntries_ZeroEventsStillMarksProcessed(t *testing.T) {
	svc, _, sessionID := newSegmentTestService(t, nil)
	ctx := context.Background()

	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hello"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "thinking"}}},
	}
	require.NoError(t, svc.GenerateSegmentEntries(ctx, sessionID, 0, 0, 0, 2, msgs))

	rows, err := svc.ProcessedSegments(ctx, sessionID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, SegmentProcessed, rows[0].State)
}

func TestGenerateSegmentEntries_ConcurrentClosesNoDuplicateKeys(t *testing.T) {
	svc, _, sessionID := newSegmentTestService(t, nil)
	ctx := context.Background()

	var wg sync.WaitGroup
	for seg := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			require.NoError(t, svc.GenerateSegmentEntries(ctx, sessionID, 0, int64(seg), int64(seg*2), int64(seg*2+2), segmentEditMsgs()))
		}()
	}
	wg.Wait()

	entries, err := svc.GetEntries(ctx, sessionID)
	require.NoError(t, err)
	require.Len(t, entries, 4)
	seen := map[[2]int64]bool{}
	for _, e := range entries {
		k := [2]int64{e.TurnNumber, e.EventNumber}
		require.False(t, seen[k], "duplicate (turn, event) pair %v", k)
		seen[k] = true
	}
}

func TestRecordSegmentClose_Idempotent(t *testing.T) {
	svc, _, sessionID := newSegmentTestService(t, nil)
	ctx := context.Background()

	require.NoError(t, svc.RecordSegmentClose(ctx, sessionID, 0, 0, 0, 5))
	require.NoError(t, svc.RecordSegmentClose(ctx, sessionID, 0, 0, 0, 5))
	rows, err := svc.ProcessedSegments(ctx, sessionID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, SegmentUnprocessed, rows[0].State)
	require.Equal(t, int64(5), rows[0].EndIndex)
}

func TestGenerateSegmentEntries_ReentrySkipsDuplicateContent(t *testing.T) {
	svc, _, sessionID := newSegmentTestService(t, nil)
	ctx := context.Background()

	require.NoError(t, svc.GenerateSegmentEntries(ctx, sessionID, 0, 0, 0, 2, segmentEditMsgs()))
	// A second generation for the same segment — e.g. a caller that
	// bypassed the in-flight mark — must not insert duplicate
	// content under fresh event numbers.
	require.NoError(t, svc.GenerateSegmentEntries(ctx, sessionID, 0, 0, 0, 2, segmentEditMsgs()))

	entries, err := svc.GetEntries(ctx, sessionID)
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func TestDeleteEntries_CascadesSegments(t *testing.T) {
	svc, _, sessionID := newSegmentTestService(t, nil)
	ctx := context.Background()

	require.NoError(t, svc.RecordSegmentClose(ctx, sessionID, 0, 0, 0, 5))
	require.NoError(t, svc.GenerateSegmentEntries(ctx, sessionID, 0, 0, 0, 5, segmentEditMsgs()))
	require.NoError(t, svc.DeleteEntries(ctx, sessionID))

	rows, err := svc.ProcessedSegments(ctx, sessionID)
	require.NoError(t, err)
	require.Empty(t, rows)
	entries, err := svc.GetEntries(ctx, sessionID)
	require.NoError(t, err)
	require.Empty(t, entries)
}
