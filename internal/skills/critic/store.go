package critic

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/google/uuid"
)

// Store persists critic reviews to the database.
type Store struct {
	q  db.Querier
	db *sql.DB
}

// NewStore creates a new critic review store backed by the given querier.
func NewStore(q db.Querier) *Store {
	return &Store{q: q}
}

// SetDB configures a raw database connection for operations that are not
// covered by the generated querier interface (e.g., bulk pruning).
func (s *Store) SetDB(db *sql.DB) {
	s.db = db
}

// DB returns the raw database connection and a boolean indicating whether it
// is configured.
func (s *Store) DB() (*sql.DB, bool) {
	return s.db, s.db != nil
}

// ReviewRecord is a domain-friendly representation of a persisted critic review.
type ReviewRecord struct {
	ID             string
	SessionID      string
	MessageID      string
	Verdict        string
	Confidence     float64
	Concerns       []CriticConcern
	Summary        string
	DiffSnapshot   string
	LSPDiagnostics []DiagnosticSnapshot
	CreatedAt      int64
}

// Create persists a critic review for the given message.
func (s *Store) Create(
	ctx context.Context,
	sessionID string,
	messageID string,
	feedback *CriticFeedback,
	diff string,
	diags []DiagnosticSnapshot,
) (ReviewRecord, error) {
	if s.q == nil {
		return ReviewRecord{}, fmt.Errorf("no querier configured")
	}

	concernsJSON, err := json.Marshal(feedback.Concerns)
	if err != nil {
		return ReviewRecord{}, fmt.Errorf("failed to marshal concerns: %w", err)
	}

	diagsJSON, err := json.Marshal(diags)
	if err != nil {
		return ReviewRecord{}, fmt.Errorf("failed to marshal diagnostics: %w", err)
	}

	review, err := s.q.CreateCriticReview(ctx, db.CreateCriticReviewParams{
		ID:             uuid.New().String(),
		SessionID:      sessionID,
		MessageID:      messageID,
		Verdict:        feedback.Verdict,
		Confidence:     feedback.Confidence,
		Concerns:       string(concernsJSON),
		Summary:        feedback.Summary,
		DiffSnapshot:   diff,
		LspDiagnostics: string(diagsJSON),
	})
	if err != nil {
		return ReviewRecord{}, fmt.Errorf("failed to create critic review: %w", err)
	}

	return s.fromDB(review)
}

// GetByMessageID retrieves the critic review associated with a message.
func (s *Store) GetByMessageID(ctx context.Context, messageID string) (ReviewRecord, error) {
	if s.q == nil {
		return ReviewRecord{}, fmt.Errorf("no querier configured")
	}
	review, err := s.q.GetCriticReviewByMessageID(ctx, messageID)
	if err != nil {
		return ReviewRecord{}, fmt.Errorf("failed to get critic review: %w", err)
	}
	return s.fromDB(review)
}

// ListBySession retrieves all critic reviews for a session, newest first.
func (s *Store) ListBySession(ctx context.Context, sessionID string) ([]ReviewRecord, error) {
	if s.q == nil {
		return nil, fmt.Errorf("no querier configured")
	}
	rows, err := s.q.ListCriticReviewsBySession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to list critic reviews: %w", err)
	}
	records := make([]ReviewRecord, len(rows))
	for i, row := range rows {
		r, err := s.fromDB(row)
		if err != nil {
			return nil, err
		}
		records[i] = r
	}
	return records, nil
}

// Prune deletes critic reviews older than the given cutoff and returns the
// number of rows removed. If no raw DB is configured, it returns an error.
func (s *Store) Prune(ctx context.Context, cutoff time.Time) (int64, error) {
	if s.db == nil {
		return 0, fmt.Errorf("no database connection configured for pruning")
	}
	result, err := s.db.ExecContext(ctx, "DELETE FROM critic_reviews WHERE created_at < ?", cutoff.Unix())
	if err != nil {
		return 0, fmt.Errorf("prune critic reviews: %w", err)
	}
	return result.RowsAffected()
}

// UpdateOutcome sets the revision outcome for the review with the given ID.
// Valid outcomes: "pending", "approved", "halted", "max_iterations".
func (s *Store) UpdateOutcome(ctx context.Context, id string, outcome string) error {
	if s.db == nil {
		return fmt.Errorf("no database connection configured")
	}
	_, err := s.db.ExecContext(ctx, "UPDATE critic_reviews SET revision_outcome = ? WHERE id = ?", outcome, id)
	if err != nil {
		return fmt.Errorf("update critic review outcome: %w", err)
	}
	return nil
}

func (s *Store) fromDB(r db.CriticReview) (ReviewRecord, error) {
	var concerns []CriticConcern
	if r.Concerns != "" {
		if err := json.Unmarshal([]byte(r.Concerns), &concerns); err != nil {
			return ReviewRecord{}, fmt.Errorf("failed to unmarshal concerns: %w", err)
		}
	}

	var diags []DiagnosticSnapshot
	if r.LspDiagnostics != "" {
		if err := json.Unmarshal([]byte(r.LspDiagnostics), &diags); err != nil {
			return ReviewRecord{}, fmt.Errorf("failed to unmarshal diagnostics: %w", err)
		}
	}

	return ReviewRecord{
		ID:             r.ID,
		SessionID:      r.SessionID,
		MessageID:      r.MessageID,
		Verdict:        r.Verdict,
		Confidence:     r.Confidence,
		Concerns:       concerns,
		Summary:        r.Summary,
		DiffSnapshot:   r.DiffSnapshot,
		LSPDiagnostics: diags,
		CreatedAt:      r.CreatedAt,
	}, nil
}
