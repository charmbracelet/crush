package notebook

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

// recordingCheckpointGen echoes per-event entries and records the
// rendered consolidation input of every GenerateCheckpoint call.
type recordingCheckpointGen struct {
	inputs []string
	text   string
}

func (g *recordingCheckpointGen) Generate(_ context.Context, _ string, events []EntryInput) ([]GeneratedEntry, error) {
	entries := make([]GeneratedEntry, len(events))
	for i, ev := range events {
		entries[i] = GeneratedEntry{
			EventType: ev.EventType,
			Title:     ev.Title,
			Text:      "## " + ev.Title + "\ncontent-" + ev.Title,
		}
	}
	return entries, nil
}

func (g *recordingCheckpointGen) GenerateCheckpoint(_ context.Context, _ string, input string) (GeneratedEntry, error) {
	g.inputs = append(g.inputs, input)
	text := g.text
	if text == "" {
		text = "## Checkpoint\n\n### Established\n- fact\n\n### Open\n- question"
	}
	return GeneratedEntry{EventType: EventCheckpoint, Title: "Checkpoint", Text: text}, nil
}

// viewMsgs builds one finished significant view call — a non-mutating
// exploration event for the checkpoint floor and tail.
func viewMsgs(id string) []message.Message {
	return []message.Message{
		{Role: message.Assistant, Parts: []message.ContentPart{
			message.ToolCall{ID: id, Name: "view", Input: `{"file_path":"auth.go"}`, Finished: true},
		}},
		{Role: message.Tool, Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: id, Name: "view", Content: strings.Repeat("x", 2000)},
		}},
	}
}

// editMsgs builds one finished mutating edit call.
func editMsgs(id string) []message.Message {
	return []message.Message{
		{Role: message.Assistant, Parts: []message.ContentPart{
			message.ToolCall{ID: id, Name: "edit", Input: `{"file_path":"auth.go"}`, Finished: true},
		}},
		{Role: message.Tool, Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: id, Name: "edit", Content: "ok"},
		}},
	}
}

func TestGenerateCheckpoint_WritesStructuredEntry(t *testing.T) {
	svc, _, sessionID := newTestService(t, &recordingCheckpointGen{})

	committed, err := svc.GenerateCheckpoint(context.Background(), sessionID, CheckpointRequest{
		TurnNumber:     1,
		SegmentNumber:  2,
		Granularity:    GranularityBoundary,
		RunTag:         "run:7",
		MinExploration: 1,
		Msgs:           viewMsgs("tc1"),
	})
	require.NoError(t, err)
	require.True(t, committed)

	entries, err := svc.GetEntries(context.Background(), sessionID)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	e := entries[0]
	require.Equal(t, EventCheckpoint, e.EventType)
	require.Equal(t, int64(1), e.TurnNumber)
	require.Equal(t, int64(2), e.SegmentNumber)
	require.Equal(t, GranularityBoundary, CheckpointGranularity(e))
	require.Contains(t, e.Tags, "run:7")
	require.Contains(t, e.Tags, "phase:checkpoint")
	require.Contains(t, e.EntryText, "Established")
	require.Contains(t, e.EntryText, "Open")
}

func TestGenerateCheckpoint_RunTagDedup(t *testing.T) {
	svc, _, sessionID := newTestService(t, &recordingCheckpointGen{})
	req := CheckpointRequest{
		TurnNumber:     1,
		SegmentNumber:  1,
		Granularity:    GranularityBoundary,
		RunTag:         "run:9",
		MinExploration: 1,
		Msgs:           viewMsgs("tc1"),
	}
	committed, err := svc.GenerateCheckpoint(context.Background(), sessionID, req)
	require.NoError(t, err)
	require.True(t, committed)

	// A retry under the same run tag is a no-op.
	committed, err = svc.GenerateCheckpoint(context.Background(), sessionID, req)
	require.NoError(t, err)
	require.False(t, committed)

	entries, err := svc.GetEntries(context.Background(), sessionID)
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func TestGenerateCheckpoint_FloorCountsOnlyNonMutating(t *testing.T) {
	svc, _, sessionID := newTestService(t, &recordingCheckpointGen{})

	// A mutating tail alone never meets the exploration floor.
	committed, err := svc.GenerateCheckpoint(context.Background(), sessionID, CheckpointRequest{
		TurnNumber:     1,
		SegmentNumber:  1,
		Granularity:    GranularityBoundary,
		RunTag:         "run:1",
		MinExploration: 1,
		Msgs:           editMsgs("tc1"),
	})
	require.NoError(t, err)
	require.False(t, committed)
}

func TestGenerateCheckpoint_InputIsCumulative(t *testing.T) {
	gen := &recordingCheckpointGen{}
	svc, _, sessionID := newTestService(t, gen)
	ctx := context.Background()

	// Seed an established fact as a committed entry on turn 1.
	require.NoError(t, svc.GenerateSegmentEntries(ctx, sessionID, 1, 1, 0, 2, viewMsgs("tc1")))

	// Checkpoint #1 consolidates it.
	committed, err := svc.GenerateCheckpoint(ctx, sessionID, CheckpointRequest{
		TurnNumber:     1,
		SegmentNumber:  1,
		Granularity:    GranularityBoundary,
		RunTag:         "run:1",
		MinExploration: 1,
		Msgs:           viewMsgs("tc2"),
	})
	require.NoError(t, err)
	require.True(t, committed)

	// Checkpoint #2 under a new run: the pre-checkpoint entry still
	// feeds the input (cumulative — the position restates all
	// established facts) but checkpoint #1's own text must not.
	committed, err = svc.GenerateCheckpoint(ctx, sessionID, CheckpointRequest{
		TurnNumber:     2,
		SegmentNumber:  1,
		Granularity:    GranularityBoundary,
		RunTag:         "run:2",
		MinExploration: 1,
		Msgs:           viewMsgs("tc3"),
	})
	require.NoError(t, err)
	require.True(t, committed)
	require.Len(t, gen.inputs, 2)
	require.Contains(t, gen.inputs[1], "content-Read auth.go")
	require.NotContains(t, gen.inputs[1], "### Established")
}

func TestGenerateCheckpoint_FinerGrainFeedsCoarser(t *testing.T) {
	gen := &recordingCheckpointGen{}
	svc, _, sessionID := newTestService(t, gen)
	ctx := context.Background()

	// A turn-grain checkpoint exists from an earlier consolidation.
	committed, err := svc.GenerateCheckpoint(ctx, sessionID, CheckpointRequest{
		TurnNumber:     1,
		SegmentNumber:  1,
		Granularity:    GranularityTurn,
		RunTag:         "run:1",
		MinExploration: 1,
		Msgs:           viewMsgs("tc1"),
	})
	require.NoError(t, err)
	require.True(t, committed)

	// The boundary checkpoint's input includes the finer turn digest.
	committed, err = svc.GenerateCheckpoint(ctx, sessionID, CheckpointRequest{
		TurnNumber:     2,
		SegmentNumber:  1,
		Granularity:    GranularityBoundary,
		RunTag:         "run:2",
		MinExploration: 1,
		Msgs:           viewMsgs("tc2"),
	})
	require.NoError(t, err)
	require.True(t, committed)
	require.Len(t, gen.inputs, 2)
	require.Contains(t, gen.inputs[1], "### Established")
}

func TestGenerateCheckpoint_FloorMeasuredSinceLastCheckpoint(t *testing.T) {
	svc, _, sessionID := newTestService(t, &recordingCheckpointGen{})
	ctx := context.Background()

	require.NoError(t, svc.GenerateSegmentEntries(ctx, sessionID, 1, 1, 0, 2, viewMsgs("tc1")))

	// First checkpoint consolidates the turn-1 entry.
	committed, err := svc.GenerateCheckpoint(ctx, sessionID, CheckpointRequest{
		TurnNumber:     1,
		SegmentNumber:  1,
		Granularity:    GranularityBoundary,
		RunTag:         "run:1",
		MinExploration: 1,
		Msgs:           nil,
	})
	require.NoError(t, err)
	require.True(t, committed)

	// Nothing gathered since — the same committed entry is below the
	// cutoff, so a higher floor must refuse to rewrite the position.
	committed, err = svc.GenerateCheckpoint(ctx, sessionID, CheckpointRequest{
		TurnNumber:     2,
		SegmentNumber:  1,
		Granularity:    GranularityBoundary,
		RunTag:         "run:2",
		MinExploration: 2,
		Msgs:           nil,
	})
	require.NoError(t, err)
	require.False(t, committed)
}

func TestTurnsWithEntries_CheckpointDoesNotCover(t *testing.T) {
	svc, _, sessionID := newTestService(t, &recordingCheckpointGen{})
	ctx := context.Background()

	committed, err := svc.GenerateCheckpoint(ctx, sessionID, CheckpointRequest{
		TurnNumber:     3,
		SegmentNumber:  1,
		Granularity:    GranularityBoundary,
		RunTag:         "run:1",
		MinExploration: 1,
		Msgs:           viewMsgs("tc1"),
	})
	require.NoError(t, err)
	require.True(t, committed)

	turns, err := svc.TurnsWithEntries(ctx, sessionID)
	require.NoError(t, err)
	require.NotContains(t, turns, int64(3),
		"a checkpoint-only turn is not a covered turn")
}
