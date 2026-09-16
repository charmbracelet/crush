package notebook

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/db"
)

// checkpointInputMaxBytes bounds the rendered consolidation input —
// committed entry digests plus tail event descriptions. Newest
// entries win the budget: the checkpoint restates the current
// position, and the oldest history is the most likely to already be
// consolidated into an earlier finer-grain entry.
const checkpointInputMaxBytes = 48_000

// GenerateCheckpoint implements the Service interface.
//
// The checkpoint consolidates "what is established, with evidence"
// versus "what is still open" into one entry the model can consult
// instead of re-deriving the investigation from raw history. Input is
// cumulative: every committed entry in the session feeds it — the
// checkpoint restates the whole position, so facts established before
// the previous checkpoint must remain reachable (finer-grain
// checkpoints feed it, same-or-coarser ones never do — the checkpoint
// replaces them rather than stacking on them) — plus classified
// events for the uncovered tail. The newest same-or-coarser
// checkpoint is only a floor marker: entries at or below it do not
// count toward the gathered threshold.
//
// Two checks run before the small-model call: the run tag dedup (a
// checkpoint already written under req.RunTag) and the gathered-count
// floor (req.MinExploration, in classified non-trivial non-mutating
// event units — consciously different units from the scope gate's
// raw call count). The entry commits inside the same write
// transaction that allocates its event number, matching segment
// generation's collision discipline; a tag re-check inside the
// transaction closes the claim/land gap between concurrent
// generations.
func (s *service) GenerateCheckpoint(ctx context.Context, sessionID string, req CheckpointRequest) (bool, error) {
	entries, err := s.GetEntries(ctx, sessionID)
	if err != nil {
		return false, err
	}

	// The cutoff is the newest same-or-coarser checkpoint: entries at
	// or below it are already consolidated. gathered tallies
	// everything newer — that is what "context gathered since the
	// last checkpoint" means, and it doubles as the check that keeps
	// a no-new-work run from rewriting an identical position.
	cutoffRank := granularityRank(req.Granularity)
	var cutoffTurn, cutoffEvent int64
	haveCutoff := false
	var inputEntries []Entry
	for _, e := range entries {
		if req.RunTag != "" && slices.Contains(e.Tags, req.RunTag) {
			return false, nil
		}
		if e.EventType == EventCheckpoint {
			g := CheckpointGranularity(e)
			if granularityRank(g) >= cutoffRank {
				if !haveCutoff || e.TurnNumber > cutoffTurn ||
					(e.TurnNumber == cutoffTurn && e.EventNumber > cutoffEvent) {
					haveCutoff = true
					cutoffTurn, cutoffEvent = e.TurnNumber, e.EventNumber
				}
				continue // Same-or-coarser checkpoints never feed a checkpoint.
			}
		}
		inputEntries = append(inputEntries, e)
	}

	gathered := 0
	for _, e := range inputEntries {
		if e.TurnNumber > cutoffTurn || (e.TurnNumber == cutoffTurn && e.EventNumber > cutoffEvent) {
			gathered++
		}
	}

	significant, _ := classifyEvents(req.Msgs)
	var tailInputs []EntryInput
	for _, ev := range significant {
		// Mutating events join the input — what changed is part of
		// the position — but only non-mutating exploration counts
		// toward the floor: the floor measures investigation depth,
		// not write volume.
		if ev.ToolCall == nil || !tools.IsMutatingCall(ev.ToolCall.Name, ev.ToolCall.Input) {
			gathered++
		}
		tailInputs = append(tailInputs, ev)
	}
	if gathered < req.MinExploration {
		return false, nil
	}

	input := buildCheckpointInput(inputEntries, tailInputs, cutoffTurn, cutoffEvent)
	if input == "" {
		return false, nil
	}
	entry, err := s.generator.GenerateCheckpoint(ctx, sessionID, input)
	if err != nil {
		return false, fmt.Errorf("failed to generate checkpoint: %w", err)
	}
	entry.EventType = EventCheckpoint
	if entry.Title == "" || entry.Title == "Entry" {
		entry.Title = "Checkpoint"
	}
	// Structural tags are stamped, not generated: granularity and the
	// run dedup tag must exist regardless of what the model wrote.
	entry.Tags = append(entry.Tags, "phase:checkpoint")
	if req.Granularity != "" {
		entry.Tags = append(entry.Tags, granularityTagPrefix+req.Granularity)
	}
	if req.RunTag != "" {
		entry.Tags = append(entry.Tags, req.RunTag)
	}
	if estimateTokens(entry.Text) > s.opts.MaxEntryTokens {
		entry.Text = truncateEntry(entry.Text, s.opts.MaxEntryTokens)
	}

	err = s.withTx(ctx, func(q *db.Queries) error {
		if req.RunTag != "" {
			// Re-check under the write lock: a sibling generation
			// that claimed first may have committed while this one
			// was in the model call.
			dup, err := q.SearchNotebookByTag(ctx, db.SearchNotebookByTagParams{
				SessionID: sessionID,
				Tag:       req.RunTag,
			})
			if err != nil {
				return err
			}
			if len(dup) > 0 {
				return errCheckpointExists
			}
		}
		maxEvent, err := q.GetMaxNotebookEventNumber(ctx, db.GetMaxNotebookEventNumberParams{
			SessionID:  sessionID,
			TurnNumber: req.TurnNumber,
		})
		if err != nil {
			return err
		}
		return storeEntry(ctx, q, sessionID, req.TurnNumber, req.SegmentNumber, maxEvent+1, entry, true, "", "")
	})
	if errors.Is(err, errCheckpointExists) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to commit checkpoint: %w", err)
	}

	if err := s.Compact(ctx, sessionID); err != nil {
		slog.Error("Failed to compact notebook", "error", err)
	}
	return true, nil
}

// errCheckpointExists aborts the commit transaction when the run tag
// re-check inside withTx finds a sibling's checkpoint — it is an
// outcome, not a failure.
var errCheckpointExists = errors.New("checkpoint already exists for run")

// buildCheckpointInput renders the consolidation input: committed
// entries in chronological order, newest filling the byte budget
// first, then the raw descriptions of the uncovered tail's
// significant events. The tail is never budget-dropped — it is the
// newest evidence and the reason the checkpoint exists.
func buildCheckpointInput(entries []Entry, tail []EntryInput, cutoffTurn, cutoffEvent int64) string {
	// Walk newest-first to spend the budget on the freshest state,
	// then reverse into reading order.
	var blocks []string
	used := 0
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		var b strings.Builder
		fmt.Fprintf(&b, "## Turn %d.%d — %s (%s)\n", e.TurnNumber, e.EventNumber, e.Title, e.EventType)
		text := e.EntryTextFull
		if text == "" {
			text = e.EntryText
		}
		b.WriteString(text)
		b.WriteString("\n\n")
		if used+b.Len() > checkpointInputMaxBytes {
			break
		}
		used += b.Len()
		blocks = append(blocks, b.String())
	}
	slices.Reverse(blocks)

	var sb strings.Builder
	sb.WriteString("Committed notebook entries (oldest first):\n\n")
	if len(blocks) == 0 {
		sb.WriteString("(none)\n\n")
	}
	for _, b := range blocks {
		sb.WriteString(b)
	}
	sb.WriteString("Recent uncovered events (oldest first):\n\n")
	for _, ev := range tail {
		fmt.Fprintf(&sb, "### %s — %s\n%s\n\n", ev.EventType, ev.Title, ev.Description)
	}
	return sb.String()
}
