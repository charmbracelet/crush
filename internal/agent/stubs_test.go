package agent

import (
	"database/sql"
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/notebook"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// newNotebookTestAgent builds a sessionAgent wired to a real notebook
// service on a scratch DB, for auto-inject and selection tests.
func newNotebookTestAgent(t *testing.T) (*sessionAgent, *db.Queries, string) {
	t.Helper()
	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	q := db.New(conn)
	sess, err := session.NewService(q, conn).Create(t.Context(), "test")
	require.NoError(t, err)
	return &sessionAgent{notebook: notebook.NewService(q, nil, notebook.Options{})}, q, sess.ID
}

func insertNotebookEntry(t *testing.T, q *db.Queries, sessionID string, turn int64, eventType, fullText string, level int64, tags ...string) {
	t.Helper()
	entry, err := q.CreateNotebookEntry(t.Context(), db.CreateNotebookEntryParams{
		ID:               uuid.New().String(),
		SessionID:        sessionID,
		TurnNumber:       turn,
		EventNumber:      1,
		EventType:        eventType,
		Title:            "entry",
		EntryText:        "compressed",
		EntryTextFull:    sql.NullString{String: fullText, Valid: fullText != ""},
		TokenCount:       5,
		CompressionLevel: level,
		Succeeded:        1,
		CreatedAt:        turn,
	})
	require.NoError(t, err)
	for _, tag := range tags {
		require.NoError(t, q.CreateNotebookTag(t.Context(), db.CreateNotebookTagParams{EntryID: entry.ID, Tag: tag}))
	}
}

// TestMaybeAutoInject_SkipsSupersededRead covers the phantom-state
// hazard: a compressed read entry whose file was later edited must not
// be auto-injected with pre-edit content.
func TestMaybeAutoInject_SkipsSupersededRead(t *testing.T) {
	t.Parallel()
	agent, q, sessionID := newNotebookTestAgent(t)

	// Turn 1: compressed read of auth.go. Turn 2: successful edit.
	insertNotebookEntry(t, q, sessionID, 1, notebook.EventFileRead, "PRE-EDIT ORIGINAL", 1, "file:auth.go")
	insertNotebookEntry(t, q, sessionID, 2, notebook.EventFileEdit, "edit applied", 0, "file:auth.go")

	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{
			message.TextContent{Text: "look at internal/auth.go please"},
		}},
	}
	msg := agent.maybeAutoInject(msgs, sessionID, 10)
	if msg != nil {
		for _, part := range msg.Content {
			if tp, ok := part.(fantasy.TextPart); ok {
				require.NotContains(t, tp.Text, "PRE-EDIT ORIGINAL")
			}
		}
	}
}

// TestMaybeAutoInject_InjectsUnsupersededRead is the positive control.
func TestMaybeAutoInject_InjectsUnsupersededRead(t *testing.T) {
	t.Parallel()
	agent, q, sessionID := newNotebookTestAgent(t)

	insertNotebookEntry(t, q, sessionID, 1, notebook.EventFileRead, "ORIGINAL CONTENT", 1, "file:auth.go")

	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{
			message.TextContent{Text: "look at internal/auth.go please"},
		}},
	}
	msg := agent.maybeAutoInject(msgs, sessionID, 10)
	require.NotNil(t, msg)
	var text string
	for _, part := range msg.Content {
		if tp, ok := part.(fantasy.TextPart); ok {
			text += tp.Text
		}
	}
	require.Contains(t, text, "ORIGINAL CONTENT")
}

// newStubTestAgent builds a sessionAgent backed by a real message
// service on a scratch SQLite DB, plus a session to attach messages to.
func newStubTestAgent(t *testing.T) (*sessionAgent, message.Service, string) {
	t.Helper()
	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	q := db.New(conn)
	sessions := session.NewService(q, conn)
	sess, err := sessions.Create(t.Context(), "test")
	require.NoError(t, err)

	svc := message.NewService(q)
	return &sessionAgent{
		messages:  svc,
		stubStats: csync.NewMap[string, stubStats](),
	}, svc, sess.ID
}

func mkMsg(t *testing.T, svc message.Service, sessionID string, role message.MessageRole, parts ...message.ContentPart) message.Message {
	t.Helper()
	m, err := svc.Create(t.Context(), sessionID, message.CreateMessageParams{
		Role:  role,
		Parts: parts,
	})
	require.NoError(t, err)
	return m
}

// viewThenEdit builds a 3-turn message list: turn 0 reads a.go, turn 1
// edits it (editOK controls success), turn 2 is a bare user message.
func viewThenEdit(t *testing.T, svc message.Service, sessionID string, viewContent string, editOK bool) []message.Message {
	t.Helper()
	var msgs []message.Message
	msgs = append(msgs, mkMsg(t, svc, sessionID, message.User, message.TextContent{Text: "start"}))
	msgs = append(msgs, mkMsg(t, svc, sessionID, message.Assistant,
		message.ToolCall{ID: "tc-view", Name: "view", Input: `{"file_path":"a.go"}`, Finished: true}))
	msgs = append(msgs, mkMsg(t, svc, sessionID, message.Tool,
		message.ToolResult{ToolCallID: "tc-view", Name: "view", Content: viewContent}))
	msgs = append(msgs, mkMsg(t, svc, sessionID, message.User, message.TextContent{Text: "edit it"}))
	msgs = append(msgs, mkMsg(t, svc, sessionID, message.Assistant,
		message.ToolCall{ID: "tc-edit", Name: "edit", Input: `{"file_path":"a.go"}`, Finished: true}))
	msgs = append(msgs, mkMsg(t, svc, sessionID, message.Tool,
		message.ToolResult{ToolCallID: "tc-edit", Name: "edit", Content: "edited", IsError: !editOK}))
	msgs = append(msgs, mkMsg(t, svc, sessionID, message.User, message.TextContent{Text: "next"}))
	return msgs
}

func bigContent() string {
	return strings.Repeat("file content line\n", 30) // ~540 bytes.
}

func resultOf(t *testing.T, m message.Message, callID string) message.ToolResult {
	t.Helper()
	for _, tr := range m.ToolResults() {
		if tr.ToolCallID == callID {
			return tr
		}
	}
	t.Fatalf("no tool result for %s", callID)
	return message.ToolResult{}
}

func TestFlagSupersededViewResults(t *testing.T) {
	t.Parallel()

	t.Run("successful edit flags earlier view", func(t *testing.T) {
		t.Parallel()
		a, svc, sessionID := newStubTestAgent(t)
		msgs := viewThenEdit(t, svc, sessionID, bigContent(), true)

		a.flagSupersededViewResults(t.Context(), msgs)

		tr := resultOf(t, msgs[2], "tc-view")
		require.NotNil(t, tr.Superseded)

		stored, err := svc.Get(t.Context(), msgs[2].ID)
		require.NoError(t, err)
		mark := resultOf(t, stored, "tc-view").Superseded
		require.NotNil(t, mark)
		require.Equal(t, "a.go", mark.Path)
		require.Equal(t, "edit", mark.ByTool)
		require.Equal(t, int64(1), mark.Turn)
		require.False(t, mark.Applied)
	})

	t.Run("failed edit does not flag", func(t *testing.T) {
		t.Parallel()
		a, svc, sessionID := newStubTestAgent(t)
		msgs := viewThenEdit(t, svc, sessionID, bigContent(), false)

		a.flagSupersededViewResults(t.Context(), msgs)

		stored, err := svc.Get(t.Context(), msgs[2].ID)
		require.NoError(t, err)
		require.Nil(t, resultOf(t, stored, "tc-view").Superseded)
	})

	t.Run("re-read after edit is not flagged", func(t *testing.T) {
		t.Parallel()
		a, svc, sessionID := newStubTestAgent(t)
		var msgs []message.Message
		// Turn 0: edit a.go first, then re-read it in the same turn.
		msgs = append(msgs, mkMsg(t, svc, sessionID, message.User, message.TextContent{Text: "go"}))
		msgs = append(msgs, mkMsg(t, svc, sessionID, message.Assistant,
			message.ToolCall{ID: "tc-edit", Name: "edit", Input: `{"file_path":"a.go"}`, Finished: true}))
		msgs = append(msgs, mkMsg(t, svc, sessionID, message.Tool,
			message.ToolResult{ToolCallID: "tc-edit", Name: "edit", Content: "edited"}))
		msgs = append(msgs, mkMsg(t, svc, sessionID, message.Assistant,
			message.ToolCall{ID: "tc-view", Name: "view", Input: `{"file_path":"a.go"}`, Finished: true}))
		msgs = append(msgs, mkMsg(t, svc, sessionID, message.Tool,
			message.ToolResult{ToolCallID: "tc-view", Name: "view", Content: bigContent()}))

		a.flagSupersededViewResults(t.Context(), msgs)

		stored, err := svc.Get(t.Context(), msgs[4].ID)
		require.NoError(t, err)
		require.Nil(t, resultOf(t, stored, "tc-view").Superseded,
			"a read that follows the write is fresh ground truth")
	})

	t.Run("small results are not flagged", func(t *testing.T) {
		t.Parallel()
		a, svc, sessionID := newStubTestAgent(t)
		msgs := viewThenEdit(t, svc, sessionID, "tiny", true)

		a.flagSupersededViewResults(t.Context(), msgs)

		stored, err := svc.Get(t.Context(), msgs[2].ID)
		require.NoError(t, err)
		require.Nil(t, resultOf(t, stored, "tc-view").Superseded)
	})

	t.Run("error results are not flagged", func(t *testing.T) {
		t.Parallel()
		a, svc, sessionID := newStubTestAgent(t)
		msgs := viewThenEdit(t, svc, sessionID, bigContent(), true)
		// Mark the view result as an error and persist.
		m := msgs[2]
		m.Parts[0] = message.ToolResult{ToolCallID: "tc-view", Name: "view", Content: bigContent(), IsError: true}
		require.NoError(t, svc.Update(t.Context(), m))

		a.flagSupersededViewResults(t.Context(), msgs)

		stored, err := svc.Get(t.Context(), m.ID)
		require.NoError(t, err)
		require.Nil(t, resultOf(t, stored, "tc-view").Superseded)
	})
}

func TestPromoteSupersededStubs(t *testing.T) {
	t.Parallel()

	t.Run("promotes pending flags inside window past recency guard", func(t *testing.T) {
		t.Parallel()
		a, svc, sessionID := newStubTestAgent(t)
		msgs := viewThenEdit(t, svc, sessionID, bigContent(), true)
		// Turn 3 — the flagged read at turn 0 is old enough.
		msgs = append(msgs, mkMsg(t, svc, sessionID, message.User, message.TextContent{Text: "later"}))
		a.flagSupersededViewResults(t.Context(), msgs)

		a.promoteSupersededStubs(t.Context(), msgs, 0)

		mark := resultOf(t, msgs[2], "tc-view").Superseded
		require.NotNil(t, mark)
		require.True(t, mark.Applied)
		stored, err := svc.Get(t.Context(), msgs[2].ID)
		require.NoError(t, err)
		require.True(t, resultOf(t, stored, "tc-view").Superseded.Applied)
	})

	t.Run("leaves flags in the last two turns pending", func(t *testing.T) {
		t.Parallel()
		a, svc, sessionID := newStubTestAgent(t)
		// Read at turn 2, superseding edit at turn 3, then promote at
		// currentTurn=4 → the read sits inside the last-two-turns
		// guard and must stay pending.
		var rebuilt []message.Message
		rebuilt = append(rebuilt, mkMsg(t, svc, sessionID, message.User, message.TextContent{Text: "t0"}))
		rebuilt = append(rebuilt, mkMsg(t, svc, sessionID, message.User, message.TextContent{Text: "t1"}))
		rebuilt = append(rebuilt, mkMsg(t, svc, sessionID, message.User, message.TextContent{Text: "t2"}))
		rebuilt = append(rebuilt, mkMsg(t, svc, sessionID, message.Assistant,
			message.ToolCall{ID: "tc-view2", Name: "view", Input: `{"file_path":"b.go"}`, Finished: true}))
		rebuilt = append(rebuilt, mkMsg(t, svc, sessionID, message.Tool,
			message.ToolResult{ToolCallID: "tc-view2", Name: "view", Content: bigContent()}))
		rebuilt = append(rebuilt, mkMsg(t, svc, sessionID, message.User, message.TextContent{Text: "t3"}))
		rebuilt = append(rebuilt, mkMsg(t, svc, sessionID, message.Assistant,
			message.ToolCall{ID: "tc-edit2", Name: "edit", Input: `{"file_path":"b.go"}`, Finished: true}))
		rebuilt = append(rebuilt, mkMsg(t, svc, sessionID, message.Tool,
			message.ToolResult{ToolCallID: "tc-edit2", Name: "edit", Content: "edited"}))
		a.flagSupersededViewResults(t.Context(), rebuilt)

		// currentTurn=4, read at turn 2 → protected (2 >= 4-2).
		a.promoteSupersededStubs(t.Context(), rebuilt, 0)

		mark := resultOf(t, rebuilt[4], "tc-view2").Superseded
		require.NotNil(t, mark)
		require.False(t, mark.Applied)
	})

	t.Run("flags before the boundary are left alone", func(t *testing.T) {
		t.Parallel()
		a, svc, sessionID := newStubTestAgent(t)
		msgs := viewThenEdit(t, svc, sessionID, bigContent(), true)
		msgs = append(msgs, mkMsg(t, svc, sessionID, message.User, message.TextContent{Text: "later"}))
		a.flagSupersededViewResults(t.Context(), msgs)

		// Boundary past the flagged result: it lives in notebook
		// territory now and is never rendered raw anyway.
		a.promoteSupersededStubs(t.Context(), msgs, 3)

		mark := resultOf(t, msgs[2], "tc-view").Superseded
		require.NotNil(t, mark)
		require.False(t, mark.Applied)
	})
}

func TestApplySupersededStubs(t *testing.T) {
	t.Parallel()

	pending := message.ToolResult{
		ToolCallID: "tc-1", Name: "view", Content: bigContent(),
		Superseded: &message.SupersededMark{Path: "a.go", ByTool: "edit", Turn: 3},
	}
	applied := pending
	applied.ToolCallID = "tc-2"
	markApplied := *pending.Superseded
	markApplied.Applied = true
	applied.Superseded = &markApplied
	failed := message.ToolResult{
		ToolCallID: "tc-3", Name: "bash", Content: bigContent(), IsError: true,
		Superseded: &message.SupersededMark{Path: "a.go", ByTool: "edit", Turn: 3, Applied: true},
	}

	m := message.Message{Role: message.Tool, Parts: []message.ContentPart{pending, applied, failed}}
	out, count, saved := applySupersededStubs(m)

	require.Equal(t, 1, count)
	require.Positive(t, saved)
	results := out.ToolResults()
	require.Len(t, results, 3)
	// Pending flag stays verbatim.
	require.Equal(t, bigContent(), results[0].Content)
	// Applied flag renders the stub.
	require.Contains(t, results[1].Content, "superseded by edit at turn 3")
	require.Contains(t, results[1].Content, "result:tc-2")
	// Error results never stub.
	require.Equal(t, bigContent(), results[2].Content)
	// Original untouched.
	require.Equal(t, bigContent(), m.ToolResults()[1].Content)
}

func TestSameFilePath(t *testing.T) {
	t.Parallel()

	require.True(t, sameFilePath("a.go", "a.go"))
	require.True(t, sameFilePath("./x.go", "x.go"))
	// Different directories — the file that exists is resolved
	// against the working dir, not suffix-matched.
	require.False(t, sameFilePath("internal/x.go", "x.go"))
	require.False(t, sameFilePath("x.go", "internal/x.go"))
	require.False(t, sameFilePath("a/x.go", "b/x.go"))
	require.False(t, sameFilePath("x.go", "y.go"))
}
