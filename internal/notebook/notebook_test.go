package notebook

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// mockGenerator implements the Generator interface for testing.
type mockGenerator struct {
	entries []GeneratedEntry
	err     error
}

func (m *mockGenerator) Generate(ctx context.Context, sessionID string, events []EntryInput) ([]GeneratedEntry, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.entries, nil
}

// newTestService creates a notebook service backed by an in-memory
// SQLite database for testing.
func newTestService(t *testing.T, gen Generator) (Service, *db.Queries, string) {
	t.Helper()
	dataDir := t.TempDir()
	conn, err := db.Connect(context.Background(), dataDir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Release(dataDir) })
	q := db.New(conn)
	// Create a session so the FK constraint on notebook_entries is
	// satisfied.
	sessionID := uuid.New().String()
	_, err = q.CreateSession(context.Background(), db.CreateSessionParams{
		ID:    sessionID,
		Title: "test",
	})
	require.NoError(t, err)
	if gen == nil {
		gen = &mockGenerator{entries: []GeneratedEntry{
			{
				EventType: EventFileRead,
				Title:     "Read auth.go",
				Text:      "## Read auth.go\n- internal/middleware/auth.go (120 lines)\n#file:auth.go #phase:exploration",
				Tags:      []string{"file:auth.go", "phase:exploration"},
			},
		}}
	}
	svc := NewService(q, gen, Options{
		MaxEntryTokens:    1000,
		MaxNotebookTokens: 100000,
	})
	return svc, q, sessionID
}

func TestClassifyEvents_SignificantAndTrivial(t *testing.T) {
	msgs := []message.Message{
		{
			Role: message.Assistant,
			Parts: []message.ContentPart{
				message.ToolCall{ID: "tc1", Name: "view", Input: `{"file_path":"auth.go"}`, Finished: true},
				message.ToolCall{ID: "tc2", Name: "grep", Input: `{"pattern":"foo"}`, Finished: true},
			},
		},
		{
			Role: message.Tool,
			Parts: []message.ContentPart{
				message.ToolResult{ToolCallID: "tc1", Name: "view", Content: string(make([]byte, 2000))},
				message.ToolResult{ToolCallID: "tc2", Name: "grep", Content: "8 results"},
			},
		},
	}
	significant, trivial := classifyEvents(msgs)
	require.Len(t, significant, 1)
	require.Len(t, trivial, 1)
	require.Equal(t, "view", significant[0].ToolCall.Name)
	require.Equal(t, "grep", trivial[0].ToolCall.Name)
}

func TestClassifyEvents_EditAlwaysSignificant(t *testing.T) {
	msgs := []message.Message{
		{
			Role: message.Assistant,
			Parts: []message.ContentPart{
				message.ToolCall{ID: "tc1", Name: "edit", Input: `{"file_path":"auth.go"}`, Finished: true},
			},
		},
		{
			Role: message.Tool,
			Parts: []message.ContentPart{
				message.ToolResult{ToolCallID: "tc1", Name: "edit", Content: "ok"},
			},
		},
	}
	significant, trivial := classifyEvents(msgs)
	require.Len(t, significant, 1)
	require.Empty(t, trivial)
}

func TestGenerateEntries_TrivialTurnSkipped(t *testing.T) {
	svc, _, sessionID := newTestService(t, nil)

	// No tool calls, no decision → skip entirely.
	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hello"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "hi there"}}},
	}
	err := svc.GenerateEntries(context.Background(), sessionID, 1, msgs)
	require.NoError(t, err)
	entries, err := svc.GetEntries(context.Background(), sessionID)
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestGenerateEntries_WithSignificantEvent(t *testing.T) {
	svc, _, sessionID := newTestService(t, nil)

	msgs := []message.Message{
		{Role: message.Assistant, Parts: []message.ContentPart{
			message.ToolCall{ID: "tc1", Name: "edit", Input: `{"file_path":"auth.go"}`, Finished: true},
		}},
		{Role: message.Tool, Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "tc1", Name: "edit", Content: "ok"},
		}},
	}
	err := svc.GenerateEntries(context.Background(), sessionID, 1, msgs)
	require.NoError(t, err)
	entries, err := svc.GetEntries(context.Background(), sessionID)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "Read auth.go", entries[0].Title)
	require.Contains(t, entries[0].Tags, "file:auth.go")
}

func TestSearchByTag(t *testing.T) {
	svc, _, sessionID := newTestService(t, nil)

	msgs := []message.Message{
		{Role: message.Assistant, Parts: []message.ContentPart{
			message.ToolCall{ID: "tc1", Name: "edit", Input: `{"file_path":"auth.go"}`, Finished: true},
		}},
		{Role: message.Tool, Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "tc1", Name: "edit", Content: "ok"},
		}},
	}
	err := svc.GenerateEntries(context.Background(), sessionID, 1, msgs)
	require.NoError(t, err)

	results, err := svc.SearchByTag(context.Background(), sessionID, "file:auth.go")
	require.NoError(t, err)
	require.Len(t, results, 1)

	results, err = svc.SearchByTag(context.Background(), sessionID, "file:nonexistent.go")
	require.NoError(t, err)
	require.Empty(t, results)
}

func TestSearchByText(t *testing.T) {
	svc, _, sessionID := newTestService(t, nil)

	msgs := []message.Message{
		{Role: message.Assistant, Parts: []message.ContentPart{
			message.ToolCall{ID: "tc1", Name: "edit", Input: `{"file_path":"auth.go"}`, Finished: true},
		}},
		{Role: message.Tool, Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "tc1", Name: "edit", Content: "ok"},
		}},
	}
	err := svc.GenerateEntries(context.Background(), sessionID, 1, msgs)
	require.NoError(t, err)

	results, err := svc.SearchByText(context.Background(), sessionID, "auth.go")
	require.NoError(t, err)
	require.Len(t, results, 1)
}

func TestSearchByEventType(t *testing.T) {
	svc, _, sessionID := newTestService(t, nil)

	msgs := []message.Message{
		{Role: message.Assistant, Parts: []message.ContentPart{
			message.ToolCall{ID: "tc1", Name: "edit", Input: `{"file_path":"auth.go"}`, Finished: true},
		}},
		{Role: message.Tool, Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "tc1", Name: "edit", Content: "ok"},
		}},
	}
	err := svc.GenerateEntries(context.Background(), sessionID, 1, msgs)
	require.NoError(t, err)

	results, err := svc.SearchByEventType(context.Background(), sessionID, EventFileRead)
	require.NoError(t, err)
	require.Len(t, results, 1)

	results, err = svc.SearchByEventType(context.Background(), sessionID, EventCommand)
	require.NoError(t, err)
	require.Empty(t, results)
}

func TestGetTokenCount(t *testing.T) {
	svc, _, sessionID := newTestService(t, nil)

	msgs := []message.Message{
		{Role: message.Assistant, Parts: []message.ContentPart{
			message.ToolCall{ID: "tc1", Name: "edit", Input: `{"file_path":"auth.go"}`, Finished: true},
		}},
		{Role: message.Tool, Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "tc1", Name: "edit", Content: "ok"},
		}},
	}
	err := svc.GenerateEntries(context.Background(), sessionID, 1, msgs)
	require.NoError(t, err)

	count, err := svc.GetTokenCount(context.Background(), sessionID)
	require.NoError(t, err)
	require.Greater(t, count, int64(0))
}

func TestDeleteEntries(t *testing.T) {
	svc, _, sessionID := newTestService(t, nil)

	msgs := []message.Message{
		{Role: message.Assistant, Parts: []message.ContentPart{
			message.ToolCall{ID: "tc1", Name: "edit", Input: `{"file_path":"auth.go"}`, Finished: true},
		}},
		{Role: message.Tool, Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "tc1", Name: "edit", Content: "ok"},
		}},
	}
	err := svc.GenerateEntries(context.Background(), sessionID, 1, msgs)
	require.NoError(t, err)

	err = svc.DeleteEntries(context.Background(), sessionID)
	require.NoError(t, err)
	entries, err := svc.GetEntries(context.Background(), sessionID)
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestRenderEntries(t *testing.T) {
	entries := []Entry{
		{Title: "Read auth.go", EntryText: "## Read auth.go\ncontent", Tags: []string{"file:auth.go"}, CompressionLevel: 0},
		{Title: "Edit auth.go", EntryText: "## Edit auth.go\ncontent", Tags: []string{"file:auth.go"}, CompressionLevel: 0},
	}
	rendered := RenderEntries(entries)
	require.Contains(t, rendered, "Read auth.go")
	require.Contains(t, rendered, "Edit auth.go")
}

func TestRenderEntries_NilEntries(t *testing.T) {
	rendered := RenderEntries(nil)
	require.Equal(t, "", rendered)
}

func TestCompact_NoOpUnderLimit(t *testing.T) {
	svc, _, sessionID := newTestService(t, nil)

	msgs := []message.Message{
		{Role: message.Assistant, Parts: []message.ContentPart{
			message.ToolCall{ID: "tc1", Name: "edit", Input: `{"file_path":"auth.go"}`, Finished: true},
		}},
		{Role: message.Tool, Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "tc1", Name: "edit", Content: "ok"},
		}},
	}
	err := svc.GenerateEntries(context.Background(), sessionID, 1, msgs)
	require.NoError(t, err)

	err = svc.Compact(context.Background(), sessionID)
	require.NoError(t, err)

	entries, err := svc.GetEntries(context.Background(), sessionID)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, int64(CompressionFull), entries[0].CompressionLevel)
}

func TestExtractPathFromInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"file_path", `{"file_path":"internal/middleware/auth.go"}`, "internal/middleware/auth.go"},
		{"path", `{"path":"config.go"}`, "config.go"},
		{"file", `{"file":"main.go"}`, "main.go"},
		{"no_match", `{"command":"ls"}`, ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPathFromInput(tt.input)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestIsSignificant(t *testing.T) {
	tests := []struct {
		name   string
		tc     message.ToolCall
		result *message.ToolResult
		want   bool
	}{
		{"view large", message.ToolCall{Name: "view"}, &message.ToolResult{Content: string(make([]byte, 2000))}, true},
		{"view small", message.ToolCall{Name: "view"}, &message.ToolResult{Content: "short"}, false},
		{"edit", message.ToolCall{Name: "edit"}, &message.ToolResult{Content: ""}, true},
		{"bash", message.ToolCall{Name: "bash"}, &message.ToolResult{Content: ""}, true},
		{"grep", message.ToolCall{Name: "grep"}, &message.ToolResult{Content: "results"}, false},
		{"glob", message.ToolCall{Name: "glob"}, &message.ToolResult{Content: "results"}, false},
		{"unknown", message.ToolCall{Name: "custom"}, &message.ToolResult{Content: ""}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSignificant(tt.tc, tt.result)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestGenerateEntries_DecisionAlongsideSignificantEvents(t *testing.T) {
	gen := &mockGenerator{entries: []GeneratedEntry{
		{
			EventType: EventFileEdit,
			Title:     "Edit auth.go",
			Text:      "## Edit auth.go\ncontent",
			Tags:      []string{"file:auth.go"},
		},
		{
			EventType: EventDecision,
			Title:     "Decision",
			Text:      "## Decision\nDecided to use JWT.",
			Tags:      []string{"phase:decision"},
		},
	}}
	svc, _, sessionID := newTestService(t, gen)

	msgs := []message.Message{
		{Role: message.Assistant, Parts: []message.ContentPart{
			message.TextContent{Text: "I decided to use JWT for auth. Let me edit auth.go."},
			message.ToolCall{ID: "tc1", Name: "edit", Input: `{"file_path":"auth.go"}`, Finished: true},
		}},
		{Role: message.Tool, Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "tc1", Name: "edit", Content: "ok"},
		}},
	}
	err := svc.GenerateEntries(context.Background(), sessionID, 1, msgs)
	require.NoError(t, err)
	entries, err := svc.GetEntries(context.Background(), sessionID)
	require.NoError(t, err)
	// Should have 2 entries: the edit + the decision.
	require.Len(t, entries, 2)
}

func TestGenerateEntries_DecisionOnlyTurn(t *testing.T) {
	gen := &mockGenerator{entries: []GeneratedEntry{
		{
			EventType: EventDecision,
			Title:     "Decision",
			Text:      "## Decision\nDecided to defer auth.",
			Tags:      []string{"phase:decision"},
		},
	}}
	svc, _, sessionID := newTestService(t, gen)

	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "should I use JWT?"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "I decided to defer auth for now."}}},
	}
	err := svc.GenerateEntries(context.Background(), sessionID, 1, msgs)
	require.NoError(t, err)
	entries, err := svc.GetEntries(context.Background(), sessionID)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, EventDecision, entries[0].EventType)
}

func TestExtractTags_SkipsMarkdownHeadings(t *testing.T) {
	text := "## Read auth.go\n- content\n### Tags\n#file:auth.go #phase:exploration"
	tags := extractTags(text)
	require.Contains(t, tags, "file:auth.go")
	require.Contains(t, tags, "phase:exploration")
	// Should not include markdown headings.
	for _, tag := range tags {
		require.False(t, strings.HasPrefix(tag, "##"))
		require.False(t, strings.HasPrefix(tag, "###"))
	}
}

func TestExtractTags_Deduplicates(t *testing.T) {
	text := "#file:auth.go #phase:exploration\n#file:auth.go"
	tags := extractTags(text)
	require.Len(t, tags, 2)
}

func TestExtractTags_StripsHashPrefix(t *testing.T) {
	// The model emits tags with # prefix (per the prompt template).
	// extractTags should strip the # so stored tags match queries
	// without #.
	text := "## Read auth.go\n### Tags\n#file:auth.go #phase:exploration"
	tags := extractTags(text)
	require.Contains(t, tags, "file:auth.go")
	require.Contains(t, tags, "phase:exploration")
	// No tag should have a # prefix.
	for _, tag := range tags {
		require.False(t, strings.HasPrefix(tag, "#"), "tag should not have # prefix: %s", tag)
	}
}

func TestComputeMaxTokens(t *testing.T) {
	// Min clamp.
	require.Equal(t, int64(2000), computeMaxTokens(0, 1000))
	require.Equal(t, int64(2000), computeMaxTokens(1, 1000))
	// Scales with event count: 5 events * 1000 * 5/4 = 6250.
	require.Equal(t, int64(6250), computeMaxTokens(5, 1000))
	// Max clamp.
	require.Equal(t, int64(16000), computeMaxTokens(100, 1000))
}

func TestDefaultTagsForEvent_UsesBasename(t *testing.T) {
	event := EntryInput{
		EventType: EventFileEdit,
		ToolCall:  &message.ToolCall{Input: `{"file_path":"internal/middleware/auth.go"}`},
	}
	tags := defaultTagsForEvent(event)
	require.Contains(t, tags, "phase:file_edit")
	require.Contains(t, tags, "file:auth.go")
	// Should NOT contain the full path.
	for _, tag := range tags {
		require.False(t, strings.Contains(tag, "internal/middleware"))
	}
}

func TestCompact_OldestFirstIncremental(t *testing.T) {
	// Use a moderate max tokens so phase 1 (full → summary) is
	// triggered but phase 2 (summary → tags) is not. This verifies
	// the intermediate summary level is produced.
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

	// Create a generator that produces large entries.
	gen := &mockGenerator{entries: []GeneratedEntry{
		{
			EventType: EventFileRead,
			Title:     "Read big.go",
			Text:      strings.Repeat("line of code\n", 200),
			Tags:      []string{"file:big.go"},
		},
	}}
	// Set MaxNotebookTokens high enough that only phase 1 triggers.
	// Each entry is ~500 tokens. Summary compression reduces to ~5
	// tokens. With 3 entries at full = 1500 tokens, setting limit
	// to 600 means we need to compress at least 2 entries to summary
	// to get under 600. Phase 2 (summary → tags) would only trigger
	// if still over after all are at summary level (15 tokens), which
	// won't happen.
	svc := NewService(q, gen, Options{
		MaxEntryTokens:    10000,
		MaxNotebookTokens: 600,
	})

	// Generate entries for 3 turns. Each turn produces one entry.
	for turn := int64(1); turn <= 3; turn++ {
		msgs := []message.Message{
			{Role: message.Assistant, Parts: []message.ContentPart{
				message.ToolCall{ID: "tc1", Name: "edit", Input: `{"file_path":"big.go"}`, Finished: true},
			}},
			{Role: message.Tool, Parts: []message.ContentPart{
				message.ToolResult{ToolCallID: "tc1", Name: "edit", Content: "ok"},
			}},
		}
		err := svc.GenerateEntries(context.Background(), sessionID, turn, msgs)
		require.NoError(t, err)
	}

	entries, err := svc.GetEntries(context.Background(), sessionID)
	require.NoError(t, err)
	require.NotEmpty(t, entries)

	// After phase 1, the oldest entries should be at level 1
	// (CompressionSummary), not level 2 (CompressionTagsOnly).
	// Compact runs after each GenerateEntries, so:
	// - After turn 1: 1 entry, under limit, no compaction.
	// - After turn 2: 2 entries, over limit, turn 1 → summary.
	// - After turn 3: 3 entries, over limit, turn 2 → summary.
	//   Then turn 3 is the only level-0 entry, so it also gets
	//   compressed to summary.
	turnLevels := make(map[int64]int64)
	for _, e := range entries {
		if e.TurnNumber > 0 {
			turnLevels[e.TurnNumber] = e.CompressionLevel
		}
	}
	// All turns should be at summary level (1), not tags-only (2).
	// Phase 2 should not trigger because the total is under the
	// limit after phase 1.
	for turn, level := range turnLevels {
		require.Equal(t, int64(CompressionSummary), level,
			"turn %d should be at summary level (1), got %d", turn, level)
	}

	// Total tokens should be under the limit.
	total, err := svc.GetTokenCount(context.Background(), sessionID)
	require.NoError(t, err)
	require.LessOrEqual(t, total, int64(600))
}

func TestCompact_Phase2_TagsOnly(t *testing.T) {
	// Use a very small max tokens to force both phases: full →
	// summary → tags-only. Verify the final state is level 2.
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

	gen := &mockGenerator{entries: []GeneratedEntry{
		{
			EventType: EventFileRead,
			Title:     "Read big.go",
			Text:      strings.Repeat("line of code\n", 200),
			Tags:      []string{"file:big.go"},
		},
	}}
	svc := NewService(q, gen, Options{
		MaxEntryTokens:    10000,
		MaxNotebookTokens: 10, // Extremely small — forces level 2.
	})

	for turn := int64(1); turn <= 3; turn++ {
		msgs := []message.Message{
			{Role: message.Assistant, Parts: []message.ContentPart{
				message.ToolCall{ID: "tc1", Name: "edit", Input: `{"file_path":"big.go"}`, Finished: true},
			}},
			{Role: message.Tool, Parts: []message.ContentPart{
				message.ToolResult{ToolCallID: "tc1", Name: "edit", Content: "ok"},
			}},
		}
		err := svc.GenerateEntries(context.Background(), sessionID, turn, msgs)
		require.NoError(t, err)
	}

	entries, err := svc.GetEntries(context.Background(), sessionID)
	require.NoError(t, err)

	// With an extremely small limit, earlier turns should be at
	// level 2 (tags-only) and the total should be under the limit.
	// The newest turn may remain at level 1 (summary) if the total
	// is already under the limit after compressing older turns.
	turnLevels := make(map[int64]int64)
	for _, e := range entries {
		turnLevels[e.TurnNumber] = e.CompressionLevel
	}
	// Turn 1 (oldest) should be at tags-only level (2).
	require.Equal(t, int64(CompressionTagsOnly), turnLevels[1],
		"oldest turn should be at tags-only level (2), got %d", turnLevels[1])
	// Turn 1 should be at least as compressed as turn 2.
	require.True(t, turnLevels[1] >= turnLevels[2],
		"oldest turn should be at least as compressed as turn 2, got %d vs %d",
		turnLevels[1], turnLevels[2])
	// Turn 2 should be at least as compressed as turn 3.
	require.True(t, turnLevels[2] >= turnLevels[3],
		"turn 2 should be at least as compressed as turn 3, got %d vs %d",
		turnLevels[2], turnLevels[3])

	total, err := svc.GetTokenCount(context.Background(), sessionID)
	require.NoError(t, err)
	require.LessOrEqual(t, total, int64(10))
}

func TestFindToolResult_NoDanglingPointer(t *testing.T) {
	msgs := []message.Message{
		{Role: message.Tool, Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "tc1", Name: "view", Content: "result1"},
		}},
	}
	result := findToolResult(msgs, "tc1")
	require.NotNil(t, result)
	require.Equal(t, "result1", result.Content)
}

func TestSearchMem0_NilGuards(t *testing.T) {
	// nil cfg, empty server name, and empty query should all
	// return empty results without error.
	ctx := context.Background()

	result, err := SearchMem0(ctx, nil, "mem0", "test")
	require.NoError(t, err)
	require.Equal(t, "", result)

	// These need a non-nil cfg to test the server-name and query
	// guards, but we can test the empty-query guard with nil since
	// it's checked first.
	result, err = SearchMem0(ctx, nil, "mem0", "")
	require.NoError(t, err)
	require.Equal(t, "", result)

	result, err = SearchMem0(ctx, nil, "mem0", "   ")
	require.NoError(t, err)
	require.Equal(t, "", result)
}

func TestTruncateTextToTokens(t *testing.T) {
	// Short text is returned as-is.
	short := "hello world"
	require.Equal(t, short, truncateTextToTokens(short, 1000))

	// Long text is truncated.
	long := strings.Repeat("a", 10000)
	result := truncateTextToTokens(long, 100)
	require.Less(t, len(result), len(long))
	require.True(t, strings.Contains(result, "[Results truncated"))

	// Exact boundary: 100 tokens * 4 chars = 400 chars.
	exact := strings.Repeat("b", 400)
	require.Equal(t, exact, truncateTextToTokens(exact, 100))
}

func TestMem0Sync_NilGuards(t *testing.T) {
	// Nil sync should be a no-op.
	var m *Mem0Sync
	m.SyncEntries(context.Background(), []Entry{{ID: "test"}})
	// Should not panic.

	// Empty server name should be a no-op.
	m = NewMem0Sync(nil, "", "session1")
	m.SyncEntries(context.Background(), []Entry{{ID: "test"}})

	// Empty entries should be a no-op.
	m = NewMem0Sync(nil, "mem0", "session1")
	m.SyncEntries(context.Background(), nil)
}
