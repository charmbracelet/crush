// Package ledger stores a session's distilled memory: what happened, what was
// decided, and which entries were retired. The pure policy over these rows
// lives in internal/compaction; this package owns persistence, so the render
// stays testable without a database.
package ledger

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/charmbracelet/crush/internal/compaction"
	"github.com/charmbracelet/crush/internal/db"
)

// Service persists the memory ledger for a session.
type Service interface {
	// Decay records what a bounded render could not keep, as retirement rows.
	// It runs when a checkpoint lands, so the decision about what memory is
	// still worth carrying is made by the same event that knows the window.
	// It returns how many entries were retired.
	Decay(ctx context.Context, sessionID string, budget compaction.RenderBudget) (int, error)
	// Append records entries and returns the rows actually stored, in
	// insertion order. Seq numbers are assigned here rather than by callers,
	// so the ledger's only clock cannot fork under concurrent observers.
	Append(ctx context.Context, sessionID string, entries []compaction.Entry) ([]compaction.Entry, error)
	// Ledger returns the stored entries and the watermark they were taken
	// through, as the render consumes them.
	Ledger(ctx context.Context, sessionID string) (compaction.Ledger, error)
	// Count returns how many entries a session holds, including retired ones;
	// the number surfaced to the user and the observability tools.
	Count(ctx context.Context, sessionID string) (int, error)
}

type service struct {
	q db.Querier
}

// NewService builds a ledger service over the shared query layer.
func NewService(q db.Querier) Service {
	return &service{q: q}
}

func (s *service) Append(ctx context.Context, sessionID string, entries []compaction.Entry) ([]compaction.Entry, error) {
	if len(entries) == 0 {
		return nil, nil
	}

	// One insert per entry rather than a single statement: the row count is
	// small, an observer runs at turn end when nothing else is writing, and
	// ON CONFLICT DO NOTHING makes a retried append safe rather than fatal.
	stored := make([]compaction.Entry, 0, len(entries))
	for _, e := range entries {
		next, err := s.nextSeq(ctx, sessionID)
		if err != nil {
			return nil, fmt.Errorf("ledger append: %w", err)
		}
		if e.ID == "" {
			// Content-addressed rather than time-addressed: an observer that
			// re-emits an entry while coverage is incomplete records it once,
			// not once per pass. The id is the identity of the claim, and the
			// same claim twice is one memory.
			sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%v", e.Kind, e.Text, e.Retires)))
			e.ID = fmt.Sprintf("e-%x", sum[:8])
		}
		e.Seq = next
		sources, err := encodeJSON(e.Sources)
		if err != nil {
			return nil, fmt.Errorf("ledger append: %w", err)
		}
		retires, err := encodeJSON(e.Retires)
		if err != nil {
			return nil, fmt.Errorf("ledger append: %w", err)
		}
		rows, err := s.q.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
			ID:        e.ID,
			SessionID: sessionID,
			Seq:       int64(e.Seq),
			Kind:      string(e.Kind),
			Text:      e.Text,
			Relevance: int64(e.Relevance),
			Sources:   sources,
			Retires:   retires,
			CreatedAt: time.Now().Unix(),
		})
		if err != nil {
			return nil, fmt.Errorf("ledger append: %w", err)
		}
		// A retried append conflicts on the entry id or the session seq and
		// writes nothing; that is "already recorded", not failure.
		if rows > 0 {
			stored = append(stored, e)
		}
	}
	return stored, nil
}

func (s *service) Ledger(ctx context.Context, sessionID string) (compaction.Ledger, error) {
	rows, err := s.q.ListLedgerEntries(ctx, sessionID)
	if err != nil {
		return compaction.Ledger{}, fmt.Errorf("ledger read: %w", err)
	}
	entries := make([]compaction.Entry, 0, len(rows))
	for _, row := range rows {
		e, err := rowToEntry(row)
		if err != nil {
			return compaction.Ledger{}, err
		}
		entries = append(entries, e)
	}
	watermark, err := s.q.MaxLedgerSeq(ctx, sessionID)
	if err != nil {
		return compaction.Ledger{}, fmt.Errorf("ledger watermark: %w", err)
	}
	return compaction.Ledger{Entries: entries, CoversThrough: int(watermark)}, nil
}

// markerKeep is how many coverage markers are retained. They exist to record
// that a region was read, never to render, so the oldest are retired once there
// are more than any later reader would consult.
const markerKeep = 200

func (s *service) Decay(ctx context.Context, sessionID string, budget compaction.RenderBudget) (int, error) {
	current, err := s.Ledger(ctx, sessionID)
	if err != nil {
		return 0, err
	}
	_, dropped := current.Select(budget)

	retire := make([]int, 0, len(dropped))
	for _, e := range dropped {
		// A decision that no longer fits stays in the ledger. It is the class
		// of memory this exists to keep, and a later render - on a larger
		// window, or after other entries are retired - can still carry it.
		if e.Relevance == compaction.RelevanceDecision {
			continue
		}
		retire = append(retire, e.Seq)
	}
	retire = append(retire, staleMarkers(current)...)
	if len(retire) == 0 {
		return 0, nil
	}

	if _, err := s.Append(ctx, sessionID, []compaction.Entry{{
		Kind:    compaction.KindDrop,
		Text:    "decayed: beyond the memory budget",
		Retires: retire,
	}}); err != nil {
		return 0, err
	}
	return len(retire), nil
}

// staleMarkers lists coverage markers beyond the newest markerKeep, oldest
// first. A marker is a drop that retires nothing; those that retire something
// are real history and are never aged out here.
func staleMarkers(l compaction.Ledger) []int {
	var markers []int
	for _, e := range l.Entries {
		if e.Kind == compaction.KindDrop && len(e.Retires) == 0 {
			markers = append(markers, e.Seq)
		}
	}
	if len(markers) <= markerKeep {
		return nil
	}
	return markers[:len(markers)-markerKeep]
}

func (s *service) Count(ctx context.Context, sessionID string) (int, error) {
	n, err := s.q.CountLedgerEntries(ctx, sessionID)
	if err != nil {
		return 0, fmt.Errorf("ledger count: %w", err)
	}
	return int(n), nil
}

// nextSeq reserves the next position. Sub-second observers cannot race here
// in practice; the unique index on (session_id, seq) is the guarantee this
// relies on, making the worst case a skipped row rather than a forked clock.
func (s *service) nextSeq(ctx context.Context, sessionID string) (int, error) {
	max, err := s.q.MaxLedgerSeq(ctx, sessionID)
	if err != nil {
		return 0, err
	}
	return int(max) + 1, nil
}

func rowToEntry(row db.CompactionLedger) (compaction.Entry, error) {
	e := compaction.Entry{
		Seq:       int(row.Seq),
		ID:        row.ID,
		Kind:      compaction.EntryKind(row.Kind),
		Text:      row.Text,
		Relevance: compaction.Relevance(row.Relevance),
	}
	if err := json.Unmarshal([]byte(row.Sources), &e.Sources); err != nil {
		return compaction.Entry{}, fmt.Errorf("ledger entry %s sources: %w", row.ID, err)
	}
	if err := json.Unmarshal([]byte(row.Retires), &e.Retires); err != nil {
		return compaction.Entry{}, fmt.Errorf("ledger entry %s retires: %w", row.ID, err)
	}
	return e, nil
}

func encodeJSON(v any) (string, error) {
	if v == nil {
		return "[]", nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
