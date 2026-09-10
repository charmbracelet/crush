package agent

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func TestFindTurnBoundaryByTokenBudget_EntireHistoryFits(t *testing.T) {
	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hello"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "hi"}}},
	}
	boundary := findTurnBoundaryByTokenBudget(msgs, 10000, estimateRawMessageTokens)
	require.Equal(t, 0, boundary)
}

func TestFindTurnBoundaryByTokenBudget_ExceedsBudget(t *testing.T) {
	// Create messages that exceed a small budget.
	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hello"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "hi"}}},
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "second message"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "response"}}},
	}
	// Small budget should force boundary past index 0.
	boundary := findTurnBoundaryByTokenBudget(msgs, 5, estimateRawMessageTokens)
	require.Greater(t, boundary, 0)
}

func TestFindTurnBoundaryByTokenBudget_ZeroBudget(t *testing.T) {
	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hello"}}},
	}
	boundary := findTurnBoundaryByTokenBudget(msgs, 0, estimateRawMessageTokens)
	require.Equal(t, 0, boundary)
}

func TestFindNextSafeBoundary_AtTurnEnd(t *testing.T) {
	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hello"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "hi"}, message.ToolCall{ID: "tc1", Name: "bash"}}},
		{Role: message.Tool, Parts: []message.ContentPart{message.ToolResult{ToolCallID: "tc1"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "done"}}},
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "next"}}},
	}
	// Start at index 1 (mid tool-call sequence) — should advance to
	// index 4 (after the assistant turn end at index 3).
	boundary := findNextSafeBoundary(msgs, 1)
	require.Equal(t, 4, boundary)
}

func TestFindNextSafeBoundary_NoSafeBoundary(t *testing.T) {
	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hello"}}},
		{Role: message.Tool, Parts: []message.ContentPart{message.ToolResult{}}},
	}
	// No assistant message with no tool calls — return start.
	boundary := findNextSafeBoundary(msgs, 0)
	require.Equal(t, 0, boundary)
}

func TestEstimateRawMessageTokens(t *testing.T) {
	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hello world"}}},
	}
	tokens := estimateRawMessageTokens(msgs)
	require.Equal(t, len("hello world")/4, tokens)
}

func TestEstimateRawMessageTokens_Empty(t *testing.T) {
	tokens := estimateRawMessageTokens(nil)
	require.Equal(t, 0, tokens)
}

func TestFindTurnBoundary_NeverDropsMostRecentTurn(t *testing.T) {
	// Even with a tiny budget, the boundary should not exclude
	// the most recent completed turn. It should find the start
	// of the most recent turn and include just that turn as raw.
	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hello"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "hi"}}},
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "second"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "response"}}},
	}
	// Budget of 1 token — everything exceeds it. The boundary
	// should be at index 2 (start of the most recent turn), not
	// 0 (which would send everything as raw) and not 4 (which
	// would drop the most recent turn).
	boundary := findTurnBoundaryByTokenBudget(msgs, 1, estimateRawMessageTokens)
	require.Equal(t, 2, boundary)
}

func TestFindTurnBoundary_MidTurnDoesNotSplitSequence(t *testing.T) {
	// A turn with tool calls should not be split. The boundary
	// should advance past the tool result.
	// The first turn is large (exceeds budget), the second turn
	// is small. The boundary should be after the first turn's
	// final assistant message (index 3), not mid-sequence.
	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hello"}}},
		{
			Role: message.Assistant,
			Parts: []message.ContentPart{
				message.TextContent{Text: strings.Repeat("let me check ", 20)},
				message.ToolCall{ID: "tc1", Name: "bash", Input: `{"command":"ls -la /very/long/path/to/somewhere"}`},
			},
		},
		{
			Role: message.Tool,
			Parts: []message.ContentPart{
				message.ToolResult{ToolCallID: "tc1", Content: strings.Repeat("output line\n", 20)},
			},
		},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "done with the analysis"}}},
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "next question please"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "response to next question"}}},
	}
	// Budget of 20 tokens: the first turn exceeds it, but the
	// second turn is small. The boundary should be at index 4
	// (after "done"), not mid-sequence.
	boundary := findTurnBoundaryByTokenBudget(msgs, 20, estimateRawMessageTokens)
	// Boundary should be at a safe point (after index 3, the
	// assistant "done" message).
	require.True(t, boundary >= 4, "boundary should not split tool call sequence, got %d", boundary)
	require.True(t, boundary < len(msgs), "boundary should not drop the most recent turn, got %d", boundary)
}

func TestCountUserMessages(t *testing.T) {
	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hello"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "hi"}}},
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "second"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "response"}}},
	}
	require.Equal(t, 2, countUserMessages(msgs))
}

func TestFindTurnStartByTurnNumber(t *testing.T) {
	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "turn0"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "resp0"}}},
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "turn1"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "resp1"}}},
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "turn2"}}},
	}
	require.Equal(t, 0, findTurnStartByTurnNumber(msgs, 0))
	require.Equal(t, 2, findTurnStartByTurnNumber(msgs, 1))
	require.Equal(t, 4, findTurnStartByTurnNumber(msgs, 2))
	require.Equal(t, 5, findTurnStartByTurnNumber(msgs, 3))
}

func TestFindMostRecentTurnStart(t *testing.T) {
	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "turn0"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "resp0"}}},
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "turn1"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "resp1"}}},
	}
	// Most recent user message is at index 2.
	require.Equal(t, 2, findMostRecentTurnStart(msgs))
}

func TestFindMostRecentTurnStart_NoUserMessages(t *testing.T) {
	msgs := []message.Message{
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "resp"}}},
	}
	require.Equal(t, 0, findMostRecentTurnStart(msgs))
}

func TestFindTurnBoundary_OversizedRecentTurn_FindsTurnStart(t *testing.T) {
	// When the most recent turn alone exceeds the budget, the
	// boundary should be the start of that turn (not 0 which
	// would send everything as raw).
	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "old turn"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "old response"}}},
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: strings.Repeat("huge ", 100)}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: strings.Repeat("big ", 100)}}},
	}
	// Budget of 1 token — the last turn alone exceeds it.
	// The boundary should be at index 2 (start of most recent
	// turn), not 0.
	boundary := findTurnBoundaryByTokenBudget(msgs, 1, estimateRawMessageTokens)
	require.Equal(t, 2, boundary, "boundary should be at start of most recent turn")
}

func TestFindStaleTurnBoundary_NoStaleTurns(t *testing.T) {
	// All turns before the boundary have notebook entries — no
	// adjustment needed.
	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "turn0"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "resp0"}}},
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "turn1"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "resp1"}}},
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "turn2"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "resp2"}}},
	}
	// boundaryTurn = 2 (turns 0 and 1 are before the boundary).
	// Both have entries.
	turnsWithEntries := map[int64]bool{0: true, 1: true}
	boundary := findStaleTurnBoundary(msgs, 2, turnsWithEntries)
	// No stale turns — boundary should be len(msgs) (no adjustment).
	require.Equal(t, len(msgs), boundary)
}

func TestFindStaleTurnBoundary_WithStaleTurn(t *testing.T) {
	// Turn 0 has no notebook entry (async generation hasn't
	// finished). The boundary should move back to include turn 0
	// as raw.
	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "turn0"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "resp0"}}},
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "turn1"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "resp1"}}},
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "turn2"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "resp2"}}},
	}
	// boundaryTurn = 2 (turns 0 and 1 are before the boundary).
	// Turn 0 is missing from the notebook.
	turnsWithEntries := map[int64]bool{1: true}
	boundary := findStaleTurnBoundary(msgs, 2, turnsWithEntries)
	// Boundary should move to index 0 (start of turn 0).
	require.Equal(t, 0, boundary, "boundary should move to start of stale turn 0")
}

func TestExtractCurrentTurnMessages(t *testing.T) {
	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "turn0"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "resp0"}}},
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "turn1"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "resp1"}}},
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "turn2"}}},
	}
	// Extract turn 1 (starts at index 2, ends before turn 2 at index 4).
	turnMsgs := extractCurrentTurnMessages(msgs, 2)
	require.Len(t, turnMsgs, 2)
	require.Equal(t, "turn1", turnMsgs[0].Content().Text)
}

func TestExtractCurrentTurnMessages_NoNextUser(t *testing.T) {
	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "turn0"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "resp0"}}},
	}
	// No next user message — should return everything from index 1.
	turnMsgs := extractCurrentTurnMessages(msgs, 1)
	require.Len(t, turnMsgs, 1)
}

func TestExtractCurrentTurnMessages_StartAtEnd(t *testing.T) {
	msgs := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "turn0"}}},
	}
	// Start index beyond slice — should return nil.
	turnMsgs := extractCurrentTurnMessages(msgs, 1)
	require.Nil(t, turnMsgs)
}

func TestExtractExplicitFilePaths(t *testing.T) {
	// Full paths with separators are extracted.
	refs := extractExplicitFilePaths("fix the bug in internal/middleware/auth.go")
	require.Contains(t, refs, "file:auth.go")
}

func TestExtractExplicitFilePaths_NoBareFilenames(t *testing.T) {
	// Bare filenames without separators should not match.
	refs := extractExplicitFilePaths("fix the bug in auth.go")
	require.NotContains(t, refs, "file:auth.go")
}

func TestExtractExplicitFilePaths_MultiplePaths(t *testing.T) {
	refs := extractExplicitFilePaths("edit internal/middleware/auth.go and internal/config/load.go")
	require.Contains(t, refs, "file:auth.go")
	require.Contains(t, refs, "file:load.go")
}

func TestExtractExplicitFilePaths_Deduplicates(t *testing.T) {
	refs := extractExplicitFilePaths("edit internal/middleware/auth.go and internal/auth.go")
	// Same basename should only appear once.
	count := 0
	for _, r := range refs {
		if r == "file:auth.go" {
			count++
		}
	}
	require.Equal(t, 1, count, "file:auth.go should appear only once")
}

func TestExtractExplicitFilePaths_RelativePath(t *testing.T) {
	refs := extractExplicitFilePaths("fix ./src/main.go")
	require.Contains(t, refs, "file:main.go")
}

func TestExtractExplicitFilePaths_StripsTrailingPunctuation(t *testing.T) {
	// Trailing periods, commas, etc. should be stripped.
	refs := extractExplicitFilePaths("fix internal/middleware/auth.go.")
	require.Contains(t, refs, "file:auth.go")
	require.NotContains(t, refs, "file:auth.go.")

	refs2 := extractExplicitFilePaths("edit internal/config/load.go, then test")
	require.Contains(t, refs2, "file:load.go")
	require.NotContains(t, refs2, "file:load.go,")
}
