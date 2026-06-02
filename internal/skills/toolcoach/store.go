package toolcoach

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/charmbracelet/crush/internal/db"
)

// Store persists toolcoach effectiveness data to the database.
type Store struct {
	q  db.Querier
	db *sql.DB
}

// NewStore creates a new toolcoach effectiveness store backed by the given querier.
func NewStore(q db.Querier) *Store {
	return &Store{q: q}
}

// SetDB configures a raw database connection for operations not covered by the
// generated querier interface.
func (s *Store) SetDB(db *sql.DB) {
	s.db = db
}

// DB returns the raw database connection and a boolean indicating whether it
// is configured.
func (s *Store) DB() (*sql.DB, bool) {
	return s.db, s.db != nil
}

// RecordSessionEffectiveness persists per-pattern effectiveness for a session.
func (s *Store) RecordSessionEffectiveness(
	ctx context.Context,
	sessionID string,
	patternID string,
	fired, acted, ignored int64,
) error {
	if s.q == nil {
		return fmt.Errorf("no querier configured")
	}
	_, err := s.q.CreateToolcoachEffectiveness(ctx, db.CreateToolcoachEffectivenessParams{
		PatternID:    patternID,
		SessionID:    sessionID,
		FiredCount:   fired,
		ActedCount:   acted,
		IgnoredCount: ignored,
	})
	if err != nil {
		return fmt.Errorf("failed to record toolcoach effectiveness: %w", err)
	}
	return nil
}

// EffectivenessRecord holds aggregated effectiveness for a single pattern.
type EffectivenessRecord struct {
	PatternID    string
	TotalFired   int64
	TotalActed   int64
	TotalIgnored int64
}

// GetPatternEffectiveness returns aggregated effectiveness for a pattern over
// the given lookback period. If the pattern has no data, it returns a zero
// record and nil error.
func (s *Store) GetPatternEffectiveness(
	ctx context.Context,
	patternID string,
	lookback time.Duration,
) (EffectivenessRecord, error) {
	if s.q == nil {
		return EffectivenessRecord{}, fmt.Errorf("no querier configured")
	}
	cutoff := time.Now().Add(-lookback).Unix()
	rows, err := s.q.GetToolcoachEffectivenessByPattern(ctx, db.GetToolcoachEffectivenessByPatternParams{
		PatternID: patternID,
		CreatedAt: cutoff,
	})
	if err != nil {
		return EffectivenessRecord{}, fmt.Errorf("failed to get toolcoach effectiveness: %w", err)
	}
	if len(rows) == 0 {
		return EffectivenessRecord{PatternID: patternID}, nil
	}
	row := rows[0]
	rec := EffectivenessRecord{PatternID: patternID}
	if row.TotalFired.Valid {
		rec.TotalFired = int64(row.TotalFired.Float64)
	}
	if row.TotalActed.Valid {
		rec.TotalActed = int64(row.TotalActed.Float64)
	}
	if row.TotalIgnored.Valid {
		rec.TotalIgnored = int64(row.TotalIgnored.Float64)
	}
	return rec, nil
}

// Prune deletes effectiveness records older than the given cutoff and returns
// the number of rows removed.
func (s *Store) Prune(ctx context.Context, cutoff time.Time) (int64, error) {
	if s.db == nil {
		return 0, fmt.Errorf("no database connection configured for pruning")
	}
	result, err := s.db.ExecContext(ctx, "DELETE FROM toolcoach_effectiveness WHERE created_at < ?", cutoff.Unix())
	if err != nil {
		return 0, fmt.Errorf("prune toolcoach effectiveness: %w", err)
	}
	return result.RowsAffected()
}
