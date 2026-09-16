package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/notebook"
	"github.com/stretchr/testify/require"
)

// cpViewCall builds one finished significant view call + result — a
// non-mutating exploration event toward the checkpoint floor.
func cpViewCall(id string) []message.Message {
	return []message.Message{
		segAssistant("", message.ToolCall{ID: id, Name: "view", Input: `{"file_path":"a.go"}`, Finished: true}),
		segTool(message.ToolResult{ToolCallID: id, Name: "view", Content: strings.Repeat("x", 2000)}),
	}
}

func TestFirstMutatingResult(t *testing.T) {
	t.Parallel()
	edit := func(id string, finished bool) message.Message {
		return segAssistant("", message.ToolCall{ID: id, Name: "edit", Input: `{"file_path":"a.go"}`, Finished: finished})
	}
	okResult := func(id string) message.Message {
		return segTool(message.ToolResult{ToolCallID: id, Name: "edit", Content: "ok"})
	}
	errResult := func(id string) message.Message {
		return segTool(message.ToolResult{ToolCallID: id, Name: "edit", Content: "denied", IsError: true})
	}

	require.True(t, firstMutatingResult([]message.Message{edit("e1", true), okResult("e1")}))
	require.False(t, firstMutatingResult([]message.Message{edit("e1", true), errResult("e1")}),
		"an errored write is not a crossed boundary")
	require.False(t, firstMutatingResult([]message.Message{edit("e1", false), okResult("e1")}),
		"an unfinished call is not a crossed boundary")
	require.False(t, firstMutatingResult(cpViewCall("v1")),
		"exploration alone is not a crossed boundary")
	// A mutating bash command counts — the shared vocabulary, not the
	// write-tool list.
	require.True(t, firstMutatingResult([]message.Message{
		segAssistant("", message.ToolCall{ID: "b1", Name: "bash", Input: `{"command":"git commit -m x"}`, Finished: true}),
		segTool(message.ToolResult{ToolCallID: "b1", Name: "bash", Content: "done"}),
	}))
}

func TestCheckpointSegmentKey_PrefersLastClosed(t *testing.T) {
	t.Parallel()
	segs := []segment{
		{turn: 1, number: 1, start: 0, end: 4},
		{turn: 1, number: 2, start: 4, end: 9, open: true},
	}
	key, ok := checkpointSegmentKey(segs)
	require.True(t, ok)
	require.Equal(t, segmentKey{turn: 1, segment: 1}, key,
		"an open-segment key can never render mid-run — the closed segment wins")

	// No closed segment: the open tail is the fallback key.
	key, ok = checkpointSegmentKey([]segment{{turn: 2, number: 1, start: 0, end: 3, open: true}})
	require.True(t, ok)
	require.Equal(t, segmentKey{turn: 2, segment: 1}, key)

	_, ok = checkpointSegmentKey(nil)
	require.False(t, ok)
}

func TestCheckpointClaimLifecycle(t *testing.T) {
	t.Parallel()
	tr := &segmentTracker{}

	require.True(t, tr.claimCheckpoint(1))
	require.False(t, tr.claimCheckpoint(1), "the run claims once")
	require.False(t, tr.retryCheckpoint(1), "an in-flight generation blocks the run-end retry")

	// A clean finish keeps the slot claimed — the boundary is
	// evaluated once per run.
	tr.finishCheckpoint(1, false)
	require.False(t, tr.claimCheckpoint(1))
	require.True(t, tr.retryCheckpoint(1), "the run-end pass may retry a finished claim")
	tr.finishCheckpoint(1, false)

	// A failed finish releases the slot for the run-end fallback.
	tr2 := &segmentTracker{}
	require.True(t, tr2.claimCheckpoint(2))
	tr2.finishCheckpoint(2, true)
	require.True(t, tr2.claimCheckpoint(2), "failure releases the claim")
}

// TestMaybeCheckpointBoundary_WritesCheckpoint is the mid-run trigger:
// a run that lands its first successful write after enough exploration
// commits a boundary checkpoint keyed to the last closed segment.
func TestMaybeCheckpointBoundary_WritesCheckpoint(t *testing.T) {
	t.Parallel()

	a, _, nb, sessionID := newSegmentTestAgent(t, echoEntryGen{})
	a.notebookCheckpoint = true
	a.nbStats = csync.NewMap[string, notebook.Stats]()

	var msgs []message.Message
	msgs = append(msgs, segUser("investigate"))
	for i := range scopeGateMinExploration {
		msgs = append(msgs, cpViewCall("v"+string(rune('a'+i)))...)
	}
	msgs = append(msgs,
		segAssistant("", message.ToolCall{ID: "e1", Name: "edit", Input: `{"file_path":"a.go"}`, Finished: true}),
		segTool(message.ToolResult{ToolCallID: "e1", Name: "edit", Content: "ok"}),
	)

	segs := segmentBoundaries(msgs, a.segTokenBudget(), a.segMaxSteps())
	ctx := context.WithValue(t.Context(), tools.RunStampContextKey, uint64(5))
	a.maybeCheckpointBoundary(ctx, sessionID, msgs, segs, nil)

	entries, err := nb.GetEntries(t.Context(), sessionID)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	e := entries[0]
	require.Equal(t, notebook.EventCheckpoint, e.EventType)
	require.Equal(t, notebook.GranularityBoundary, notebook.CheckpointGranularity(e))
	require.Contains(t, e.Tags, "run:5")

	stats, ok := a.nbStats.Get(sessionID)
	require.True(t, ok)
	require.Equal(t, 1, stats.CheckpointsWritten)

	// A second detection pass under the same stamp is deduped by the
	// per-run claim.
	a.maybeCheckpointBoundary(ctx, sessionID, msgs, segs, nil)
	entries, err = nb.GetEntries(t.Context(), sessionID)
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

// TestMaybeCheckpointBoundary_NoWriteNoCheckpoint: exploration without
// a crossed write boundary never reaches generation.
func TestMaybeCheckpointBoundary_NoWriteNoCheckpoint(t *testing.T) {
	t.Parallel()

	a, _, nb, sessionID := newSegmentTestAgent(t, echoEntryGen{})
	a.notebookCheckpoint = true

	var msgs []message.Message
	msgs = append(msgs, segUser("investigate"))
	for i := range scopeGateMinExploration {
		msgs = append(msgs, cpViewCall("v"+string(rune('a'+i)))...)
	}
	segs := segmentBoundaries(msgs, a.segTokenBudget(), a.segMaxSteps())
	ctx := context.WithValue(t.Context(), tools.RunStampContextKey, uint64(6))
	a.maybeCheckpointBoundary(ctx, sessionID, msgs, segs, nil)

	entries, err := nb.GetEntries(t.Context(), sessionID)
	require.NoError(t, err)
	require.Empty(t, entries)
}

// TestGenerateRunEndCheckpoint is the fallback: a run that gathered
// context without crossing the write boundary consolidates at run
// end under a one-event floor.
func TestGenerateRunEndCheckpoint(t *testing.T) {
	t.Parallel()

	a, _, nb, sessionID := newSegmentTestAgent(t, echoEntryGen{})
	a.notebookCheckpoint = true

	msgs := append([]message.Message{segUser("look")}, cpViewCall("v1")...)
	a.generateRunEndCheckpoint(t.Context(), sessionID, msgs, 11, nil)

	entries, err := nb.GetEntries(t.Context(), sessionID)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Contains(t, entries[0].Tags, "run:11")

	// The run-tag check dedups a second run-end pass — the durable
	// form of the claim.
	a.generateRunEndCheckpoint(t.Context(), sessionID, msgs, 11, nil)
	entries, err = nb.GetEntries(t.Context(), sessionID)
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

// TestGenerateRunEndCheckpoint_EmptyRun: nothing gathered, nothing
// written — a no-new-work run must not rewrite the position.
func TestGenerateRunEndCheckpoint_EmptyRun(t *testing.T) {
	t.Parallel()

	a, _, nb, sessionID := newSegmentTestAgent(t, echoEntryGen{})
	a.notebookCheckpoint = true

	msgs := []message.Message{
		segUser("hi"),
		segAssistant("hello"),
	}
	a.generateRunEndCheckpoint(t.Context(), sessionID, msgs, 12, nil)

	entries, err := nb.GetEntries(t.Context(), sessionID)
	require.NoError(t, err)
	require.Empty(t, entries)
}
