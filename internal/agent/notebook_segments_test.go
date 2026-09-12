package agent

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/notebook"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

func segUser(text string) message.Message {
	return message.Message{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: text}}}
}

func segAssistant(text string, calls ...message.ToolCall) message.Message {
	parts := []message.ContentPart{message.TextContent{Text: text}}
	for _, c := range calls {
		parts = append(parts, c)
	}
	return message.Message{Role: message.Assistant, Parts: parts}
}

func segTool(results ...message.ToolResult) message.Message {
	parts := make([]message.ContentPart, len(results))
	for i, r := range results {
		parts[i] = r
	}
	return message.Message{Role: message.Tool, Parts: parts}
}

func TestAllCallsResolved(t *testing.T) {
	t.Parallel()

	msgs := []message.Message{
		segUser("start"),
		segAssistant("working", message.ToolCall{ID: "tc1", Name: "bash"}),
		segTool(message.ToolResult{ToolCallID: "tc1"}),
		segAssistant("done"),
	}
	// Mid tool-run: the call at index 1 resolves at index 2.
	require.False(t, allCallsResolved(msgs, 2))
	require.True(t, allCallsResolved(msgs, 3))
	require.True(t, allCallsResolved(msgs, 4))
	require.True(t, allCallsResolved(msgs, 0))
}

func TestAllCallsResolved_OrphanDoesNotBlock(t *testing.T) {
	t.Parallel()

	// A call with no result anywhere (cancelled) must not block.
	msgs := []message.Message{
		segUser("start"),
		segAssistant("working", message.ToolCall{ID: "tc-orphan", Name: "bash"}),
		segAssistant("moved on"),
	}
	require.True(t, allCallsResolved(msgs, 3))
	require.True(t, allCallsResolved(msgs, 2))
}

func TestSegmentBoundaries_TokenThreshold(t *testing.T) {
	t.Parallel()

	big := strings.Repeat("x", 4000) // ~1000 tokens per message.
	msgs := []message.Message{segUser("go")}
	for range 12 {
		msgs = append(msgs,
			segAssistant(big),
			segTool(message.ToolResult{ToolCallID: "x"}), // results with no calls are orphan-safe here.
		)
	}
	segs := segmentBoundaries(msgs, 3000, 100)
	require.GreaterOrEqual(t, len(segs), 3)
	require.True(t, segs[len(segs)-1].open)
	for i, s := range segs[:len(segs)-1] {
		require.False(t, s.open)
		require.Equal(t, s.end, segs[i+1].start)
	}
	// All segments share turn 0 — a single user turn.
	for _, s := range segs {
		require.Equal(t, int64(0), s.turn)
	}
	for i, s := range segs {
		require.Equal(t, int64(i), s.number)
	}
}

func TestSegmentBoundaries_StepCap(t *testing.T) {
	t.Parallel()

	msgs := []message.Message{segUser("go")}
	for range 15 {
		msgs = append(msgs, segAssistant("step"))
	}
	segs := segmentBoundaries(msgs, 1<<30, 5)
	// 15 assistant steps at cap 5 → 3 closed segments + open tail
	// (tail may be empty of new steps but always exists).
	require.GreaterOrEqual(t, len(segs), 3)
	closedSteps := 0
	for _, s := range segs[:len(segs)-1] {
		for _, m := range msgs[s.start:s.end] {
			if m.Role == message.Assistant {
				closedSteps++
			}
		}
	}
	require.Equal(t, 15, closedSteps+countAssistant(msgs[segs[len(segs)-1].start:]))
}

func countAssistant(msgs []message.Message) int {
	n := 0
	for _, m := range msgs {
		if m.Role == message.Assistant {
			n++
		}
	}
	return n
}

func TestSegmentBoundaries_WaitsForCallResolution(t *testing.T) {
	t.Parallel()

	big := strings.Repeat("x", 4000)
	msgs := []message.Message{
		segUser("go"),
		segAssistant(big, message.ToolCall{ID: "tc1", Name: "bash"}),
		segAssistant(big), // next step text before result lands is unusual but legal in the model.
		segTool(message.ToolResult{ToolCallID: "tc1"}),
		segAssistant(big),
	}
	// Threshold trips after message 1, but the cut at index 2/3 would
	// split tc1 from its result at index 3 — the close must wait.
	segs := segmentBoundaries(msgs, 1000, 100)
	require.Len(t, segs, 2)
	require.Equal(t, 4, segs[0].end)
	require.True(t, segs[1].open)
}

func TestSegmentBoundaries_StragglerResultDoesNotBlockUserClose(t *testing.T) {
	t.Parallel()

	// A call whose results straddle a user message — e.g. a cancelled
	// tool's late second result landing after a folded prompt — must
	// not block the user-boundary close. The straggler becomes an
	// orphan in the next segment, which the renderer drops.
	msgs := []message.Message{
		segUser("first"),
		segAssistant("a", message.ToolCall{ID: "tc1", Name: "bash"}),
		segTool(message.ToolResult{ToolCallID: "tc1"}),
		segUser("folded"),
		segAssistant("b"),
		segTool(message.ToolResult{ToolCallID: "tc1"}), // straggler
	}
	segs := segmentBoundaries(msgs, 1<<30, 100)
	require.Len(t, segs, 2)
	require.Equal(t, 3, segs[1].start)
	require.Equal(t, int64(1), segs[1].turn)
	require.Equal(t, int64(0), segs[0].turn)
}

func TestSegmentBoundaries_OrphanCallDoesNotBlockThresholdClose(t *testing.T) {
	t.Parallel()

	big := strings.Repeat("x", 4000)
	msgs := []message.Message{
		segUser("go"),
		// A cancelled call — no result lands anywhere — must not
		// block a threshold close: pending only tracks calls with a
		// known result position.
		segAssistant(big, message.ToolCall{ID: "tc-orphan", Name: "bash"}),
		segAssistant(big),
		segAssistant(big),
	}
	segs := segmentBoundaries(msgs, 2000, 100)
	require.GreaterOrEqual(t, len(segs), 2)
	for _, s := range segs[:len(segs)-1] {
		require.True(t, allCallsResolved(msgs, s.end))
	}
}

func TestSegmentBoundaries_FoldedUserMessageIsHardBoundary(t *testing.T) {
	t.Parallel()

	msgs := []message.Message{
		segUser("first"),
		segAssistant("a"),
		segUser("folded mid-run"), // hard boundary: new turn, new segment.
		segAssistant("b"),
	}
	segs := segmentBoundaries(msgs, 1<<30, 100)
	require.Len(t, segs, 2)
	require.Equal(t, 2, segs[1].start)
	require.Equal(t, int64(0), segs[0].number)
	require.Equal(t, int64(0), segs[1].number)
	require.Equal(t, int64(1), segs[1].turn)
}

func TestSegmentBoundaries_EmptyAssistantTailDoesNotFreeze(t *testing.T) {
	t.Parallel()

	big := strings.Repeat("x", 4000)
	msgs := []message.Message{
		segUser("go"),
		segAssistant(big),
		{Role: message.Assistant}, // cancelled — empty, no calls.
		segAssistant(big),
	}
	segs := segmentBoundaries(msgs, 1000, 100)
	require.GreaterOrEqual(t, len(segs), 2)
	// Every closed boundary is a safe index.
	for _, s := range segs[:len(segs)-1] {
		require.True(t, allCallsResolved(msgs, s.end))
	}
}

func TestFindSegmentBoundary_OpenAlwaysRaw(t *testing.T) {
	t.Parallel()

	msgs := []message.Message{segUser("go"), segAssistant("a")}
	segs := segmentBoundaries(msgs, 1<<30, 100)
	boundary := findSegmentBoundaryByTokenBudget(msgs, 1, segs, nil)
	require.Equal(t, 0, boundary)
}

func TestFindSegmentBoundary_StalePullBack(t *testing.T) {
	t.Parallel()

	big := strings.Repeat("x", 4000)
	var msgs []message.Message
	msgs = append(msgs, segUser("go"))
	for range 8 {
		msgs = append(msgs, segAssistant(big), segTool())
	}
	segs := segmentBoundaries(msgs, 2000, 100)
	require.GreaterOrEqual(t, len(segs), 4)

	// All closed segments processed except the oldest one: the
	// pull-back must expose it as raw rather than dropping it
	// uncovered.
	processed := map[segmentKey]bool{}
	for _, s := range segs[1 : len(segs)-1] {
		processed[s.key()] = true
	}
	boundary := findSegmentBoundaryByTokenBudget(msgs, 2000, segs, processed)
	require.Equal(t, segs[0].start, boundary)

	// Fully covered: the boundary keeps only the budget's worth of
	// closed segments plus the open tail.
	processed = map[segmentKey]bool{}
	for _, s := range segs[:len(segs)-1] {
		processed[s.key()] = true
	}
	boundary = findSegmentBoundaryByTokenBudget(msgs, 2000, segs, processed)
	require.Greater(t, boundary, 0)
	require.True(t, allCallsResolved(msgs, boundary))
}

func TestBoundarySegmentKey(t *testing.T) {
	t.Parallel()

	msgs := []message.Message{
		segUser("a"), segAssistant("x"),
		segUser("b"), segAssistant("y"),
	}
	segs := segmentBoundaries(msgs, 1<<30, 100)
	require.Equal(t, segmentKey{turn: 0, segment: 0}, boundarySegmentKey(segs, 0))
	require.Equal(t, segmentKey{turn: 1, segment: 0}, boundarySegmentKey(segs, 2))
}

func TestSegmentRetryDue(t *testing.T) {
	t.Parallel()

	now := time.Now()
	require.True(t, segmentRetryDue(notebook.ProcessedSegment{}, now))
	require.False(t, segmentRetryDue(notebook.ProcessedSegment{
		RetryCount:    1,
		LastAttemptAt: now.Add(-time.Second).Unix(),
	}, now))
	require.True(t, segmentRetryDue(notebook.ProcessedSegment{
		RetryCount:    1,
		LastAttemptAt: now.Add(-time.Hour).Unix(),
	}, now))
}

// countingGen wraps the echo generator and records invocation count.
type countingGen struct {
	calls atomic.Int64
}

func (g *countingGen) Generate(ctx context.Context, sessionID string, events []notebook.EntryInput) ([]notebook.GeneratedEntry, error) {
	g.calls.Add(1)
	return echoEntryGen{}.Generate(ctx, sessionID, events)
}

// newSegmentTestAgent builds a sessionAgent on real services with
// synchronous segment generation for determinism.
func newSegmentTestAgent(t *testing.T, gen notebook.Generator) (*sessionAgent, message.Service, notebook.Service, string) {
	t.Helper()
	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	q := db.New(conn)
	sessions := session.NewService(q, conn)
	sess, err := sessions.Create(t.Context(), "seg")
	require.NoError(t, err)
	svc := message.NewService(q)
	nb := notebook.NewService(q, gen, notebook.Options{DB: conn, MaxEntryTokens: 10000, MaxNotebookTokens: 100000})

	return &sessionAgent{
		messages:        svc,
		sessions:        sessions,
		notebook:        nb,
		notebookEnabled: true,
		syncSegmentGen:  true,
		segmentTrackers: csync.NewMap[string, *segmentTracker](),
		prefixCache:     csync.NewMap[string, cachedPrefix](),
		stubBoundary:    csync.NewMap[string, int](),
		stubStats:       csync.NewMap[string, stubStats](),
		systemPrompt:    csync.NewValue("system"),
	}, svc, nb, sess.ID
}

// segBuildTurn appends a turn of n assistant+tool steps to the service
// and returns the full stored message list.
func segBuildTurn(t *testing.T, svc message.Service, sessionID, userText string, steps int, stepText string) []message.Message {
	t.Helper()
	mkMsg(t, svc, sessionID, message.User, message.TextContent{Text: userText})
	for i := range steps {
		mkMsg(t, svc, sessionID, message.Assistant,
			message.TextContent{Text: stepText},
			message.ToolCall{ID: "tc-" + userText + "-" + strings.Repeat("x", i%3) + string(rune('a'+i)), Name: "bash", Input: `{"command":"ls"}`, Finished: true})
		mkMsg(t, svc, sessionID, message.Tool,
			message.ToolResult{ToolCallID: "tc-" + userText + "-" + strings.Repeat("x", i%3) + string(rune('a'+i)), Name: "bash", Content: stepText})
	}
	msgs, err := svc.List(t.Context(), sessionID)
	require.NoError(t, err)
	return msgs
}

func TestDetectSegments_ClosesGeneratesAndDeduplicates(t *testing.T) {
	t.Parallel()

	gen := &countingGen{}
	a, svc, nb, sessionID := newSegmentTestAgent(t, gen)
	a.segmentMaxSteps = 2

	// One user turn with 4 steps → segments of 2 steps each close as
	// the list grows past them.
	msgs := segBuildTurn(t, svc, sessionID, "work", 4, "step content")

	ctx := t.Context()
	segs, processed := a.detectSegments(ctx, sessionID, msgs)
	require.GreaterOrEqual(t, len(segs), 2)
	// The first pass fires generation; coverage is visible to the
	// boundary walk from the next pass on — the conservative pull-back
	// never drops a segment whose coverage is in flight.
	require.Empty(t, processed)

	rows, err := nb.ProcessedSegments(ctx, sessionID)
	require.NoError(t, err)
	for _, r := range rows {
		require.Equal(t, notebook.SegmentProcessed, r.State)
	}

	// Second pass sees the committed coverage.
	_, processed = a.detectSegments(ctx, sessionID, msgs)
	require.NotEmpty(t, processed)

	// Entries carry segment numbers and continue the turn's event
	// numbering — no duplicate (turn, event) pairs.
	entries, err := nb.GetEntries(ctx, sessionID)
	require.NoError(t, err)
	require.NotEmpty(t, entries)
	seen := map[[2]int64]bool{}
	for _, e := range entries {
		k := [2]int64{e.TurnNumber, e.EventNumber}
		require.False(t, seen[k], "duplicate (turn, event) pair %v", k)
		seen[k] = true
	}

	// A second detection pass re-fires nothing.
	before := gen.calls.Load()
	_, processed = a.detectSegments(ctx, sessionID, msgs)
	require.Equal(t, before, gen.calls.Load())
	require.Len(t, processed, len(rows))
}

func TestDetectSegments_PureTextSegmentMarksProcessed(t *testing.T) {
	t.Parallel()

	gen := &countingGen{}
	a, svc, _, sessionID := newSegmentTestAgent(t, gen)
	a.segmentMaxSteps = 2

	// Pure text/reasoning steps classify as zero events — the segment
	// must still be marked processed or it pins the boundary forever
	// and re-fires generation every pass.
	mkMsg(t, svc, sessionID, message.User, message.TextContent{Text: "go"})
	for range 4 {
		mkMsg(t, svc, sessionID, message.Assistant, message.TextContent{Text: "thinking out loud"})
	}
	msgs, err := svc.List(t.Context(), sessionID)
	require.NoError(t, err)

	ctx := t.Context()
	segs, _ := a.detectSegments(ctx, sessionID, msgs)
	require.GreaterOrEqual(t, len(segs), 2)

	// Coverage lands on the following pass.
	_, processed := a.detectSegments(ctx, sessionID, msgs)
	require.NotEmpty(t, processed, "zero-event segment must mark processed")

	before := gen.calls.Load()
	_, _ = a.detectSegments(ctx, sessionID, msgs)
	require.Equal(t, before, gen.calls.Load(), "covered segment must not re-fire")
}

func TestDetectSegments_BackfillsLegacyTurnWithoutRegenerating(t *testing.T) {
	t.Parallel()

	gen := &countingGen{}
	a, svc, nb, sessionID := newSegmentTestAgent(t, gen)
	a.segmentMaxSteps = 2

	// Legacy turn-grain entries: written before the registry existed.
	msgs := segBuildTurn(t, svc, sessionID, "work", 4, "step content")
	require.NoError(t, nb.GenerateEntries(t.Context(), sessionID, 0, msgs))

	ctx := t.Context()
	segs, _ := a.detectSegments(ctx, sessionID, msgs)
	require.GreaterOrEqual(t, len(segs), 2)

	// Backfill marks the turn's closed segments processed without
	// calling the generator again — visible from the next pass.
	require.Equal(t, int64(1), gen.calls.Load())
	_, processed := a.detectSegments(ctx, sessionID, msgs)
	require.NotEmpty(t, processed)
}

func TestDetectSegments_BackfillCoversLegacyOpenTail(t *testing.T) {
	t.Parallel()

	gen := &countingGen{}
	a, svc, nb, sessionID := newSegmentTestAgent(t, gen)
	a.segmentMaxSteps = 2

	// A legacy session's last turn has turn-grain entries but its
	// tail — holding the final step — is still the open segment: it
	// closes only when the next user message lands.
	msgs := segBuildTurn(t, svc, sessionID, "work", 5, "step content")
	require.NoError(t, nb.GenerateEntries(t.Context(), sessionID, 0, msgs))
	require.Equal(t, int64(1), gen.calls.Load())

	segs := segmentBoundaries(msgs, a.segTokenBudget(), a.segMaxSteps())
	require.True(t, segs[len(segs)-1].open)
	require.NotEmpty(t, msgs[segs[len(segs)-1].start:segs[len(segs)-1].end], "the open tail must hold messages")

	ctx := t.Context()
	a.detectSegments(ctx, sessionID, msgs)

	// The new user message closes the tail at exactly the extent the
	// backfill recorded, so it stays processed — regenerating would
	// duplicate coverage the turn-grain entries already provide.
	mkMsg(t, svc, sessionID, message.User, message.TextContent{Text: "next"})
	msgs, err := svc.List(ctx, sessionID)
	require.NoError(t, err)
	segs, processed := a.detectSegments(ctx, sessionID, msgs)

	require.Equal(t, int64(1), gen.calls.Load(), "closed legacy tail must not regenerate")
	for _, s := range segs[:len(segs)-1] {
		require.True(t, processed[s.key()], "segment %v should report processed coverage", s.key())
	}
}

func TestGenerateRunEndSegments_NoDuplicatesForCoveredSegments(t *testing.T) {
	t.Parallel()

	gen := &countingGen{}
	a, svc, nb, sessionID := newSegmentTestAgent(t, gen)
	a.segmentMaxSteps = 2

	msgs := segBuildTurn(t, svc, sessionID, "work", 6, "step content")
	preTurn := 0
	lastAssistant := msgs[len(msgs)-2].ID // last assistant msg

	ctx := t.Context()
	// Mid-run detection covers the closed segments; run-end must only
	// generate the tail.
	a.detectSegments(ctx, sessionID, msgs)
	callsAfterDetect := gen.calls.Load()

	a.generateRunEndSegments(ctx, sessionID, msgs, preTurn, lastAssistant)

	entries, err := nb.GetEntries(ctx, sessionID)
	require.NoError(t, err)
	seen := map[[2]int64]bool{}
	for _, e := range entries {
		k := [2]int64{e.TurnNumber, e.EventNumber}
		require.False(t, seen[k], "duplicate (turn, event) pair %v", k)
		seen[k] = true
	}

	// A repeated run-end pass fires nothing.
	before := gen.calls.Load()
	a.generateRunEndSegments(ctx, sessionID, msgs, preTurn, lastAssistant)
	require.Equal(t, before, gen.calls.Load())
	_ = callsAfterDetect
}

func TestPreparePrompt_LongTurnStaysInSegmentBand(t *testing.T) {
	t.Parallel()

	gen := &countingGen{}
	a, svc, _, sessionID := newSegmentTestAgent(t, gen)
	a.segmentMaxSteps = 6

	// One user turn, ~50 steps of ~4K chars each ≈ 200K chars total —
	// the recorded-session shape the issue describes.
	big := strings.Repeat("file content line\n", 220)
	msgs := segBuildTurn(t, svc, sessionID, "big task", 24, big)

	ctx := t.Context()
	// First pass records closes and generates coverage; the second
	// render sees committed coverage and drops covered segments.
	a.preparePrompt(ctx, msgs, false)
	history, _ := a.preparePrompt(ctx, msgs, false)
	require.NotEmpty(t, history)

	// Count raw messages in the rebuild: with default 25K-token budget
	// the raw window should hold roughly the open segment plus one or
	// two closed ones — a fraction of 49 messages.
	var rawCount int
	var sawNotebook bool
	for _, m := range history {
		if m.Role == fantasy.MessageRoleSystem {
			sawNotebook = true
			continue
		}
		rawCount++
	}
	require.True(t, sawNotebook, "covered prefix should produce a notebook system message")
	// Each step renders as assistant + tool message; the band should be
	// far below the full 49.
	require.Less(t, rawCount, 30, "raw history should stay near the 1-2 segment band")
}

func TestNotebookPrefix_ByteIdenticalOnUnmovedBoundary(t *testing.T) {
	t.Parallel()

	gen := &countingGen{}
	a, svc, nb, sessionID := newSegmentTestAgent(t, gen)
	a.segmentMaxSteps = 2

	msgs := segBuildTurn(t, svc, sessionID, "work", 4, "step content")
	ctx := t.Context()

	segs, _ := a.detectSegments(ctx, sessionID, msgs)
	processed := map[segmentKey]bool{}
	for _, s := range segs[:len(segs)-1] {
		processed[s.key()] = true
	}
	boundary := findSegmentBoundaryByTokenBudget(msgs, 1, segs, processed)
	bKey := boundarySegmentKey(segs, boundary)

	// Seed an entry so the prefix is non-empty.
	require.NoError(t, nb.GenerateSegmentEntries(ctx, sessionID, segs[0].turn, segs[0].number, int64(segs[0].start), int64(segs[0].end), msgs[segs[0].start:segs[0].end]))

	first := a.notebookPrefix(ctx, sessionID, msgs, boundary, bKey, segs)
	second := a.notebookPrefix(ctx, sessionID, msgs, boundary, bKey, segs)
	require.NotEmpty(t, first)
	require.Equal(t, first, second, "unmoved boundary must render a byte-identical prefix")
}

func TestPromoteSupersededStubs_SegmentRecencyGuard(t *testing.T) {
	t.Parallel()

	a, svc, _, sessionID := newSegmentTestAgent(t, echoEntryGen{})
	a.stubSuperseded = true
	a.segmentMaxSteps = 1 // one segment per step for a tight guard.

	ctx := t.Context()
	// Build: read a.go, then edit a.go in an OLD segment, then several
	// plain steps so the read falls outside the recency guard.
	mkMsg(t, svc, sessionID, message.User, message.TextContent{Text: "go"})
	mkMsg(t, svc, sessionID, message.Assistant,
		message.ToolCall{ID: "tc-read", Name: "view", Input: `{"file_path":"a.go"}`, Finished: true})
	mkMsg(t, svc, sessionID, message.Tool,
		message.ToolResult{ToolCallID: "tc-read", Name: "view", Content: bigContent()})
	mkMsg(t, svc, sessionID, message.Assistant,
		message.ToolCall{ID: "tc-edit", Name: "edit", Input: `{"file_path":"a.go"}`, Finished: true})
	mkMsg(t, svc, sessionID, message.Tool,
		message.ToolResult{ToolCallID: "tc-edit", Name: "edit", Content: "ok"})
	for range 4 {
		mkMsg(t, svc, sessionID, message.Assistant, message.TextContent{Text: "step"})
	}
	msgs, err := svc.List(ctx, sessionID)
	require.NoError(t, err)

	a.flagPrunableToolResults(ctx, msgs)
	segs := segmentBoundaries(msgs, a.segTokenBudget(), a.segMaxSteps())
	require.GreaterOrEqual(t, len(segs), 4)

	// Promote with boundary 0 (whole window) — the read's mark is in an
	// old segment and must flip.
	require.True(t, a.promoteSupersededStubs(ctx, msgs, 0, segs))
	stored, err := svc.Get(ctx, msgs[2].ID)
	require.NoError(t, err)
	res := stored.ToolResults()[0]
	require.NotNil(t, res.Superseded)
	require.True(t, res.Superseded.Applied, "stale read in an old segment should promote on a boundary move")
}

// TestRenderNotebookPrefix_SegmentCoverageFilter exercises the
// (turn, segment) coverage compare: entries from the boundary's own
// segment or later must not render into the prefix.
func TestRenderNotebookPrefix_SegmentCoverageFilter(t *testing.T) {
	t.Parallel()

	a := &sessionAgent{}
	entries := []notebook.Entry{
		nbSegEntry("e00", 0, 0, 1, notebook.EventGeneral, "segment zero content", 10),
		nbSegEntry("e01", 0, 1, 1, notebook.EventGeneral, "segment one content", 10),
		nbSegEntry("e02", 0, 2, 1, notebook.EventGeneral, "segment two content", 10),
	}
	rawMsgs := []message.Message{segUser("go"), segAssistant("work")}
	prefix := a.renderNotebookPrefix(t.Context(), "sess", entries, rawMsgs,
		segmentKey{turn: 0, segment: 2}, segmentKey{turn: 0, segment: 0}, nil)
	require.Len(t, prefix, 1)
	require.Equal(t, fantasy.MessageRoleSystem, prefix[0].Role)
	text := prefix[0].Content[0].(fantasy.TextPart).Text
	require.Contains(t, text, "segment zero content")
	require.Contains(t, text, "segment one content")
	require.NotContains(t, text, "segment two content")
}

// TestDetectSegments_AsyncCloses exercises the real goroutine path —
// multiple segments firing generation and flagging concurrently. Run
// with -race to cover the per-goroutine clone isolation.
func TestDetectSegments_AsyncCloses(t *testing.T) {
	t.Parallel()

	a, svc, nb, sessionID := newSegmentTestAgent(t, echoEntryGen{})
	a.stubSuperseded = true
	a.syncSegmentGen = false
	a.segmentMaxSteps = 2

	// Read then edit the same file across segments so flagging has
	// work to do concurrently with generation.
	mkMsg(t, svc, sessionID, message.User, message.TextContent{Text: "go"})
	for i := range 6 {
		name := "view"
		if i%2 == 1 {
			name = "edit"
		}
		mkMsg(t, svc, sessionID, message.Assistant,
			message.ToolCall{ID: "tc" + string(rune('a'+i)), Name: name, Input: `{"file_path":"a.go"}`, Finished: true})
		mkMsg(t, svc, sessionID, message.Tool,
			message.ToolResult{ToolCallID: "tc" + string(rune('a'+i)), Name: name, Content: bigContent()})
	}
	msgs, err := svc.List(t.Context(), sessionID)
	require.NoError(t, err)

	a.detectSegments(t.Context(), sessionID, msgs)
	require.Eventually(t, func() bool {
		rows, err := nb.ProcessedSegments(t.Context(), sessionID)
		if err != nil {
			return false
		}
		for _, r := range rows {
			if r.State != notebook.SegmentProcessed {
				return false
			}
		}
		return len(rows) > 0
	}, 10*time.Second, 10*time.Millisecond, "closed segments should reach processed")
}

func TestRebuildStepMessages_PreservesSystemAndTail(t *testing.T) {
	t.Parallel()

	gen := &countingGen{}
	a, svc, _, sessionID := newSegmentTestAgent(t, gen)
	a.segmentMaxSteps = 2

	msgs := segBuildTurn(t, svc, sessionID, "work", 6, "step content")
	require.NoError(t, svc.FlushAll(t.Context()))

	// options.Messages as Fantasy builds them: system + stored history.
	options := []fantasy.Message{fantasy.NewSystemMessage("sys")}
	for _, m := range msgs {
		options = append(options, m.ToAIMessage()...)
	}

	out, ok := a.rebuildStepMessages(t.Context(), sessionID, options, false)
	require.True(t, ok)
	require.Equal(t, fantasy.MessageRoleSystem, out[0].Role)
	require.NotEmpty(t, out)
}

// TestRebuildStepMessages_TailByteIdentical is the tail-fidelity check:
// with no coverage committed (boundary 0), the rebuilt list must equal
// Fantasy's accumulated list part-for-part.
func TestRebuildStepMessages_TailByteIdentical(t *testing.T) {
	t.Parallel()

	a, svc, _, sessionID := newSegmentTestAgent(t, echoEntryGen{})
	a.segmentMaxSteps = 100 // nothing closes — boundary stays 0.

	msgs := segBuildTurn(t, svc, sessionID, "work", 4, "step content")
	require.NoError(t, svc.FlushAll(t.Context()))

	options := []fantasy.Message{fantasy.NewSystemMessage("sys")}
	for _, m := range msgs {
		options = append(options, m.ToAIMessage()...)
	}

	out, ok := a.rebuildStepMessages(t.Context(), sessionID, options, false)
	require.True(t, ok)
	require.Len(t, out, len(options))
	for i := range options {
		require.True(t, fantasyMessageEqual(options[i], out[i]),
			"rebuilt message %d diverges from Fantasy's accumulated message", i)
	}
}

// TestSegmentBoundaries_StableUnderAppliedStubs is the drift
// regression: promoting a superseded mark inside a segment must not
// shift its recomputed close point, or coverage and content would
// silently disagree.
func TestSegmentBoundaries_StableUnderAppliedStubs(t *testing.T) {
	t.Parallel()

	big := strings.Repeat("x", 4000)
	build := func(applied bool) []message.Message {
		res := message.ToolResult{ToolCallID: "tc1", Name: "view", Content: big}
		res.Superseded = &message.SupersededMark{Path: "a.go", ByTool: "edit", Applied: applied}
		var msgs []message.Message
		msgs = append(msgs, segUser("go"))
		msgs = append(msgs, segAssistant(big, message.ToolCall{ID: "tc1", Name: "view"}))
		msgs = append(msgs, segTool(res))
		for range 8 {
			msgs = append(msgs, segAssistant(big))
		}
		return msgs
	}

	verbatim := segmentBoundaries(build(false), 2000, 3)
	stubbed := segmentBoundaries(build(true), 2000, 3)
	require.Equal(t, verbatim, stubbed, "applied stubs must not shift segment boundaries")
}
