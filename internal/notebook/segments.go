package notebook

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
)

// Segment coverage states recorded in the processed_segments registry.
const (
	// SegmentUnprocessed is written at segment close: the extent is
	// durable from the moment the segment closes, but its notebook
	// entries have not landed yet.
	SegmentUnprocessed = "unprocessed"
	// SegmentProcessed is written on completion in the same
	// transaction as the segment's entries. Only processed segments
	// count as covered: an unprocessed segment stays raw until its
	// coverage lands, so nothing drops without entries behind it.
	SegmentProcessed = "processed"
)

// ProcessedSegment is one row of the segment coverage registry: the
// recorded extent of a closed segment and whether its notebook entries
// have been generated. Extents are recorded at close time and never
// change — identity-by-range keeps coverage correct even when stored
// message content later mutates (stub promotion, sanitization).
type ProcessedSegment struct {
	TurnNumber    int64
	SegmentNumber int64
	StartIndex    int64
	EndIndex      int64
	State         string
	RetryCount    int64
	LastAttemptAt int64
}

// RecordSegmentClose records a freshly closed segment's extent as
// unprocessed. It is idempotent: segment detection re-runs every step,
// so the insert is INSERT OR IGNORE under the (session, turn, segment)
// primary key. Callers fire generation separately.
func (s *service) RecordSegmentClose(ctx context.Context, sessionID string, turnNumber, segmentNumber, startIndex, endIndex int64) error {
	return s.q.RecordProcessedSegment(ctx, db.RecordProcessedSegmentParams{
		SessionID:     sessionID,
		TurnNumber:    turnNumber,
		SegmentNumber: segmentNumber,
		StartIndex:    startIndex,
		EndIndex:      endIndex,
		CreatedAt:     time.Now().Unix(),
	})
}

// ProcessedSegments lists every recorded segment for a session, in
// (turn, segment) order.
func (s *service) ProcessedSegments(ctx context.Context, sessionID string) ([]ProcessedSegment, error) {
	rows, err := s.q.ListProcessedSegments(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to list processed segments: %w", err)
	}
	out := make([]ProcessedSegment, 0, len(rows))
	for _, r := range rows {
		out = append(out, ProcessedSegment{
			TurnNumber:    r.TurnNumber,
			SegmentNumber: r.SegmentNumber,
			StartIndex:    r.StartIndex,
			EndIndex:      r.EndIndex,
			State:         r.State,
			RetryCount:    r.RetryCount,
			LastAttemptAt: r.LastAttemptAt.Int64,
		})
	}
	return out, nil
}

// MarkSegmentsProcessed inserts coverage rows already in the processed
// state — the backfill path for pre-upgrade sessions whose turns
// already have turn-grain entries. No generation is fired.
func (s *service) MarkSegmentsProcessed(ctx context.Context, sessionID string, segs []ProcessedSegment) error {
	write := func(q *db.Queries) error {
		now := time.Now().Unix()
		for _, seg := range segs {
			if err := q.RecordProcessedSegment(ctx, db.RecordProcessedSegmentParams{
				SessionID:     sessionID,
				TurnNumber:    seg.TurnNumber,
				SegmentNumber: seg.SegmentNumber,
				StartIndex:    seg.StartIndex,
				EndIndex:      seg.EndIndex,
				CreatedAt:     now,
			}); err != nil {
				return err
			}
			if err := q.MarkSegmentProcessed(ctx, db.MarkSegmentProcessedParams{
				SessionID:     sessionID,
				TurnNumber:    seg.TurnNumber,
				SegmentNumber: seg.SegmentNumber,
			}); err != nil {
				return err
			}
		}
		return nil
	}
	return s.withTx(ctx, write)
}

// RecordSegmentAttempt bumps the retry counters when a generation
// attempt starts, so a failed attempt backs off instead of re-firing
// on every detection pass.
func (s *service) RecordSegmentAttempt(ctx context.Context, sessionID string, turnNumber, segmentNumber int64) error {
	return s.q.RecordSegmentAttempt(ctx, db.RecordSegmentAttemptParams{
		LastAttemptAt: sql.NullInt64{Int64: time.Now().Unix(), Valid: true},
		SessionID:     sessionID,
		TurnNumber:    turnNumber,
		SegmentNumber: segmentNumber,
	})
}

// GetByTurnSegment retrieves entries for one segment of a turn.
func (s *service) GetByTurnSegment(ctx context.Context, sessionID string, turnNumber, segmentNumber int64) ([]Entry, error) {
	rows, err := s.q.GetNotebookEntriesByTurnSegment(ctx, db.GetNotebookEntriesByTurnSegmentParams{
		SessionID:     sessionID,
		TurnNumber:    turnNumber,
		SegmentNumber: segmentNumber,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get notebook entries by turn/segment: %w", err)
	}
	return s.enrichEntries(ctx, rows)
}

// TurnsWithEntries returns the set of turn numbers that have at least
// one entry. Used by the migration backfill: a pre-upgrade turn with
// entries is covered at turn grain and is marked processed directly.
func (s *service) TurnsWithEntries(ctx context.Context, sessionID string) (map[int64]bool, error) {
	turns, err := s.q.GetNotebookTurnsWithEntries(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to list turns with entries: %w", err)
	}
	out := make(map[int64]bool, len(turns))
	for _, t := range turns {
		out[t] = true
	}
	return out, nil
}

// GenerateSegmentEntries classifies the events in one closed segment,
// generates entries for them, and commits entries + the processed
// marker in a single transaction so a crash can't leave entries
// without coverage (which would re-fire generation and duplicate
// them). Event numbers continue the turn's existing sequence rather
// than restarting at zero — two segments of one turn would otherwise
// emit duplicate (turn, event) keys. The offset is read inside the
// transaction so concurrent segment completions serialize on SQLite's
// write lock instead of racing a stale MAX().
//
// A segment with no significant events still commits the marker:
// success-with-zero-entries counts as covered, or a pure-text segment
// would pin the raw window forever and re-fire generation every step.
func (s *service) GenerateSegmentEntries(ctx context.Context, sessionID string, turnNumber, segmentNumber, startIndex, endIndex int64, msgs []message.Message) error {
	significant, trivial := classifyEvents(msgs)
	if hasDecision(msgs) {
		significant = append(significant, EntryInput{
			EventType:   EventDecision,
			Title:       "Decision",
			Description: extractAssistantText(msgs),
			Succeeded:   true,
		})
	}

	type pendingEntry struct {
		entry     GeneratedEntry
		succeeded bool
		headline  string
	}
	var pending []pendingEntry
	if len(trivial) > 0 {
		pending = append(pending, pendingEntry{
			entry:     buildTrivialExplorationEntry(trivial),
			succeeded: eventsSucceeded(trivial),
		})
	}
	if len(significant) > 0 {
		entries, err := s.generator.Generate(ctx, sessionID, significant)
		if err != nil {
			return fmt.Errorf("failed to generate notebook entries: %w", err)
		}
		for i, entry := range entries {
			if estimateTokens(entry.Text) > s.opts.MaxEntryTokens {
				entry.Text = truncateEntry(entry.Text, s.opts.MaxEntryTokens)
			}
			var headline string
			succeeded := true
			if i < len(significant) {
				succeeded = significant[i].Succeeded
				headline = significant[i].ErrorHeadline
			}
			pending = append(pending, pendingEntry{entry: entry, succeeded: succeeded, headline: headline})
		}
	}

	committed := false
	err := s.withTx(ctx, func(q *db.Queries) error {
		// Record the extent for the run-end tail path, whose segment
		// was never closed by mid-run detection.
		if err := q.RecordProcessedSegment(ctx, db.RecordProcessedSegmentParams{
			SessionID:     sessionID,
			TurnNumber:    turnNumber,
			SegmentNumber: segmentNumber,
			StartIndex:    startIndex,
			EndIndex:      endIndex,
			CreatedAt:     time.Now().Unix(),
		}); err != nil {
			return err
		}
		// Re-entry guard: a concurrent caller (another process, or a
		// caller that bypassed the in-flight mark) may have committed
		// this segment already — never insert duplicate content under
		// fresh event numbers.
		existing, err := q.GetProcessedSegment(ctx, db.GetProcessedSegmentParams{
			SessionID:     sessionID,
			TurnNumber:    turnNumber,
			SegmentNumber: segmentNumber,
		})
		if err != nil {
			return err
		}
		if existing.State == SegmentProcessed {
			return nil
		}
		maxEvent, err := q.GetMaxNotebookEventNumber(ctx, db.GetMaxNotebookEventNumberParams{
			SessionID:  sessionID,
			TurnNumber: turnNumber,
		})
		if err != nil {
			return err
		}
		for i, p := range pending {
			if err := storeEntry(ctx, q, sessionID, turnNumber, segmentNumber, maxEvent+1+int64(i), p.entry, p.succeeded, p.headline); err != nil {
				return err
			}
		}
		committed = true
		return q.MarkSegmentProcessed(ctx, db.MarkSegmentProcessedParams{
			SessionID:     sessionID,
			TurnNumber:    turnNumber,
			SegmentNumber: segmentNumber,
		})
	})
	if err != nil {
		return fmt.Errorf("failed to commit segment entries: %w", err)
	}
	if !committed {
		return nil
	}

	// Compact if needed.
	if err := s.Compact(ctx, sessionID); err != nil {
		slog.Error("Failed to compact notebook", "error", err)
	}
	return nil
}

// withTx runs write inside a transaction when a DB handle is
// configured; without one it falls back to sequential statements
// (tests that construct the service without a connection).
func (s *service) withTx(ctx context.Context, write func(q *db.Queries) error) error {
	if s.db == nil {
		return write(s.q)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck
	if err := write(s.q.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit()
}
