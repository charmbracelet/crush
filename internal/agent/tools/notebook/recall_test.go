package notebook

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/notebook"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

// newRecallTestEnv builds a message service on a scratch DB and returns
// it with a session ID.
func newRecallTestEnv(t *testing.T) (message.Service, string) {
	t.Helper()
	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	q := db.New(conn)
	sessions := session.NewService(q, conn)
	sess, err := sessions.Create(t.Context(), "test")
	require.NoError(t, err)
	return message.NewService(q), sess.ID
}

func runRecall(t *testing.T, tool fantasy.AgentTool, ctx context.Context, query string) fantasy.ToolResponse {
	t.Helper()
	input, err := json.Marshal(RecallParams{Query: query})
	require.NoError(t, err)
	resp, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    "test-call",
		Name:  RecallToolName,
		Input: string(input),
	})
	require.NoError(t, err)
	return resp
}

func TestRecallToolResult(t *testing.T) {
	t.Parallel()

	svc, sessionID := newRecallTestEnv(t)
	tool := NewRecallTool(nil, svc, nil, "", false, nil)
	ctx := context.WithValue(t.Context(), tools.SessionIDContextKey, sessionID)

	original := "compile failed:\nmain.go:12: undefined: foo\nmain.go:13: missing return"
	_, err := svc.Create(ctx, sessionID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{ID: "tc-1", Name: "bash", Input: `{"command":"go build ."}`, Finished: true},
		},
	})
	require.NoError(t, err)
	_, err = svc.Create(ctx, sessionID, message.CreateMessageParams{
		Role: message.Tool,
		Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "tc-1", Name: "bash", Content: original, IsError: true},
		},
	})
	require.NoError(t, err)

	t.Run("returns original full content", func(t *testing.T) {
		t.Parallel()
		resp := runRecall(t, tool, ctx, "result:tc-1")
		require.False(t, resp.IsError)
		require.Contains(t, resp.Content, "(error)")
		require.Contains(t, resp.Content, original)
	})

	t.Run("unknown call id is a clean miss", func(t *testing.T) {
		t.Parallel()
		resp := runRecall(t, tool, ctx, "result:nope")
		require.False(t, resp.IsError)
		require.Contains(t, resp.Content, "No tool result found")
	})

	t.Run("empty id is an error", func(t *testing.T) {
		t.Parallel()
		resp := runRecall(t, tool, ctx, "result:")
		require.True(t, resp.IsError)
	})
}

func TestRecallToolResult_TruncatesLargeContent(t *testing.T) {
	t.Parallel()

	svc, sessionID := newRecallTestEnv(t)
	tool := NewRecallTool(nil, svc, nil, "", false, nil)
	ctx := context.WithValue(t.Context(), tools.SessionIDContextKey, sessionID)

	_, err := svc.Create(ctx, sessionID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{ID: "tc-big", Name: "view", Input: `{"file_path":"a.go"}`, Finished: true},
		},
	})
	require.NoError(t, err)
	_, err = svc.Create(ctx, sessionID, message.CreateMessageParams{
		Role: message.Tool,
		Parts: []message.ContentPart{
			// Well over the recall cap.
			message.ToolResult{ToolCallID: "tc-big", Name: "view", Content: strings.Repeat("x", tools.MaxOutputLength*2)},
		},
	})
	require.NoError(t, err)

	resp := runRecall(t, tool, ctx, "result:tc-big")
	require.False(t, resp.IsError)
	// The marker distinguishes this read-back cut from a capture-time
	// cap already embedded in stored content.
	require.Contains(t, resp.Content, "truncated at recall")
	require.Less(t, len(resp.Content), tools.MaxOutputLength+1000)
}

// echoGenerator produces one generated entry per input event, keeping
// index alignment with the significant-event list.
type echoGenerator struct{}

func (echoGenerator) Generate(ctx context.Context, sessionID string, events []notebook.EntryInput) ([]notebook.GeneratedEntry, error) {
	entries := make([]notebook.GeneratedEntry, len(events))
	for i, ev := range events {
		entries[i] = notebook.GeneratedEntry{
			EventType: ev.EventType,
			Title:     ev.Title,
			Text:      "## " + ev.Title + "\ncontent",
			Tags:      []string{"file:a.go"},
		}
	}
	return entries, nil
}

func (echoGenerator) GenerateCheckpoint(ctx context.Context, sessionID, input string) (notebook.GeneratedEntry, error) {
	return notebook.GeneratedEntry{
		EventType: notebook.EventCheckpoint,
		Title:     "Checkpoint",
		Text:      "## Checkpoint\n\n" + input,
	}, nil
}

// newNotebookTestEnv builds a session, a real notebook service, and a
// recall tool wired to both.
func newNotebookTestEnv(t *testing.T) (notebook.Service, message.Service, string, *csync.Map[string, notebook.Stats]) {
	t.Helper()
	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	q := db.New(conn)
	sessions := session.NewService(q, conn)
	sess, err := sessions.Create(t.Context(), "test")
	require.NoError(t, err)
	svc := notebook.NewService(q, echoGenerator{}, notebook.Options{
		MaxEntryTokens:    1000,
		MaxNotebookTokens: 100000,
	})
	return svc, message.NewService(q), sess.ID, csync.NewMap[string, notebook.Stats]()
}

func TestRecallSegmentQuery(t *testing.T) {
	t.Parallel()

	svc, msgs, sessionID, stats := newNotebookTestEnv(t)
	tool := NewRecallTool(svc, msgs, nil, "", false, stats)
	ctx := context.WithValue(t.Context(), tools.SessionIDContextKey, sessionID)

	eventMsgs := []message.Message{
		{Role: message.Assistant, Parts: []message.ContentPart{
			message.ToolCall{ID: "tc-e", Name: "edit", Input: `{"file_path":"a.go"}`, Finished: true},
		}},
		{Role: message.Tool, Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "tc-e", Name: "edit", Content: "ok"},
		}},
	}
	// One entry committed as turn 1, segment 3.
	require.NoError(t, svc.GenerateSegmentEntries(t.Context(), sessionID, 1, 3, 0, 2, eventMsgs))

	t.Run("segment query returns the entry", func(t *testing.T) {
		t.Parallel()
		resp := runRecall(t, tool, ctx, "segment:1.3")
		require.False(t, resp.IsError)
		require.Contains(t, resp.Content, "Edit a.go")
	})

	t.Run("wrong segment is a clean miss", func(t *testing.T) {
		t.Parallel()
		resp := runRecall(t, tool, ctx, "segment:1.9")
		require.False(t, resp.IsError)
		require.Contains(t, resp.Content, "No notebook entries")
	})

	t.Run("malformed segment is an error", func(t *testing.T) {
		t.Parallel()
		resp := runRecall(t, tool, ctx, "segment:1")
		require.True(t, resp.IsError)
	})
}

func TestRecallStatsByQueryType(t *testing.T) {
	t.Parallel()

	svc, msgs, sessionID, stats := newNotebookTestEnv(t)
	tool := NewRecallTool(svc, msgs, nil, "", false, stats)
	ctx := context.WithValue(t.Context(), tools.SessionIDContextKey, sessionID)

	// Seed one raw tool result so result: recall has a target.
	_, err := msgs.Create(ctx, sessionID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{ID: "tc-1", Name: "bash", Input: `{"command":"go build"}`, Finished: true},
		},
	})
	require.NoError(t, err)
	_, err = msgs.Create(ctx, sessionID, message.CreateMessageParams{
		Role: message.Tool,
		Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "tc-1", Name: "bash", Content: "ok"},
		},
	})
	require.NoError(t, err)

	// result: counts against the stubbing track; everything else
	// counts against the notebook track.
	runRecall(t, tool, ctx, "result:tc-1")
	runRecall(t, tool, ctx, "turn:1")
	runRecall(t, tool, ctx, "segment:1.3")
	runRecall(t, tool, ctx, "file:a.go")

	got, ok := stats.Get(sessionID)
	require.True(t, ok)
	require.Equal(t, 1, got.ResultRecalls)
	require.Equal(t, 3, got.EntryRecalls)
	require.Equal(t, 0, got.CrossRecalls)
	// All three entry queries returned nothing — misses counted
	// separately from attempts.
	require.Equal(t, 3, got.EmptyRecalls)
}

func TestRecallCheckpointQuery(t *testing.T) {
	t.Parallel()

	svc, msgs, sessionID, stats := newNotebookTestEnv(t)
	tool := NewRecallTool(svc, msgs, nil, "", false, stats)
	ctx := context.WithValue(t.Context(), tools.SessionIDContextKey, sessionID)

	// Seed a committed checkpoint to recall.
	committed, err := svc.GenerateCheckpoint(t.Context(), sessionID, notebook.CheckpointRequest{
		TurnNumber:     1,
		SegmentNumber:  1,
		Granularity:    notebook.GranularityBoundary,
		RunTag:         "run:1",
		MinExploration: 1,
		Msgs: []message.Message{
			{Role: message.Assistant, Parts: []message.ContentPart{
				message.ToolCall{ID: "tc-v", Name: "view", Input: `{"file_path":"a.go"}`, Finished: true},
			}},
			{Role: message.Tool, Parts: []message.ContentPart{
				message.ToolResult{ToolCallID: "tc-v", Name: "view", Content: strings.Repeat("x", 2000)},
			}},
		},
	})
	require.NoError(t, err)
	require.True(t, committed)

	resp := runRecall(t, tool, ctx, "checkpoint")
	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, "Checkpoint")
}
