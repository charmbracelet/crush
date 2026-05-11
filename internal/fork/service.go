// Package fork provides conversation forking functionality.
// Forking creates a new session from a specific point in conversation history,
// optionally restoring the filesystem to that snapshot.
package fork

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/charmbracelet/crush/internal/checkpoint"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/worktree"
)

// Common errors.
var (
	ErrSnapshotNotFound = errors.New("snapshot not found")
	ErrMessageNotFound  = errors.New("message not found")
	ErrForkFailed       = errors.New("fork failed")
)

// Service handles conversation forking.
type Service interface {
	pubsub.Subscriber[ForkResult]

	// Fork creates a new session forked from a specific message.
	// If createWorktree is true, also creates a worktree with the snapshot state.
	Fork(ctx context.Context, params ForkParams) (*ForkResult, error)

	// GetForkHistory returns all sessions that were forked from a given snapshot.
	GetForkHistory(ctx context.Context, snapshotID string) ([]session.Session, error)
}

// ForkParams contains parameters for forking a conversation.
type ForkParams struct {
	// SessionID is the source session to fork from.
	SessionID string

	// MessageID is the message to fork from. The new session will include
	// all messages up to and including this one.
	MessageID string

	// CreateWorktree if true, creates a new worktree with the snapshot state.
	CreateWorktree bool

	// WorktreeName is the name for the new worktree. Auto-generated if empty.
	WorktreeName string

	// Title is the title for the new session. Auto-generated if empty.
	Title string
}

// ForkResult contains the result of a fork operation.
type ForkResult struct {
	// NewSession is the newly created session.
	NewSession session.Session

	// SourceSnapshot is the snapshot that was used for the fork.
	SourceSnapshot *checkpoint.Snapshot

	// Worktree is the newly created worktree, if any.
	Worktree *worktree.Worktree

	// CreatedAt is when the fork was created.
	CreatedAt time.Time
}

// service implements the Service interface.
type service struct {
	*pubsub.Broker[ForkResult]

	queries     *db.Queries
	conn        *sql.DB
	sessions    session.Service
	messages    message.Service
	checkpoints checkpoint.Service
	worktrees   worktree.Service
}

// NewService creates a new fork service.
func NewService(
	queries *db.Queries,
	conn *sql.DB,
	sessions session.Service,
	messages message.Service,
	checkpoints checkpoint.Service,
	worktrees worktree.Service,
) Service {
	return &service{
		Broker:      pubsub.NewBroker[ForkResult](),
		queries:     queries,
		conn:        conn,
		sessions:    sessions,
		messages:    messages,
		checkpoints: checkpoints,
		worktrees:   worktrees,
	}
}

func (s *service) Fork(ctx context.Context, params ForkParams) (*ForkResult, error) {
	// Get the snapshot for the message.
	var snapshot *checkpoint.Snapshot
	if s.checkpoints != nil && s.checkpoints.IsEnabled() {
		var err error
		snapshot, err = s.checkpoints.GetSnapshotByMessage(ctx, params.MessageID)
		if err != nil && !errors.Is(err, checkpoint.ErrSnapshotNotFound) {
			return nil, fmt.Errorf("get snapshot: %w", err)
		}
	}

	// Get source session to copy title if needed.
	sourceSession, err := s.sessions.Get(ctx, params.SessionID)
	if err != nil {
		return nil, fmt.Errorf("get source session: %w", err)
	}

	// Generate title if not provided.
	title := params.Title
	if title == "" {
		title = fmt.Sprintf("Fork of %s", sourceSession.Title)
	}

	// Create new session.
	newSession, err := s.sessions.Create(ctx, title)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	// Copy messages up to and including the specified message.
	if err := s.copyMessagesUpTo(ctx, params.SessionID, newSession.ID, params.MessageID); err != nil {
		// Clean up on failure.
		_ = s.sessions.Delete(ctx, newSession.ID)
		return nil, fmt.Errorf("copy messages: %w", err)
	}

	// Update session with fork reference.
	if snapshot != nil {
		if err := s.queries.UpdateSessionForkedFrom(ctx, db.UpdateSessionForkedFromParams{
			ForkedFromSnapshotID: sql.NullString{String: snapshot.ID, Valid: true},
			ID:                   newSession.ID,
		}); err != nil {
			// Non-fatal, just log.
		}
	}

	result := &ForkResult{
		NewSession:     newSession,
		SourceSnapshot: snapshot,
		CreatedAt:      time.Now(),
	}

	// Create worktree if requested.
	if params.CreateWorktree && s.worktrees != nil && s.worktrees.IsEnabled() {
		snapshotID := ""
		if snapshot != nil {
			snapshotID = snapshot.ID
		}

		wt, err := s.worktrees.Create(ctx, newSession.ID, params.WorktreeName, snapshotID)
		if err != nil {
			// Non-fatal, session was still created.
			// TODO: Log warning.
		} else {
			result.Worktree = wt
		}
	}

	s.Publish(pubsub.CreatedEvent, *result)

	return result, nil
}

func (s *service) GetForkHistory(ctx context.Context, snapshotID string) ([]session.Session, error) {
	// This would require a query to find all sessions with forked_from_snapshot_id = snapshotID.
	// For now, return empty since we don't have that query.
	return nil, nil
}

// copyMessagesUpTo copies all messages from source session to target session,
// up to and including the specified message.
func (s *service) copyMessagesUpTo(ctx context.Context, sourceSessionID, targetSessionID, upToMessageID string) error {
	msgs, err := s.messages.List(ctx, sourceSessionID)
	if err != nil {
		return err
	}

	for _, msg := range msgs {
		// Create a copy of the message in the new session.
		_, err := s.messages.Create(ctx, targetSessionID, message.CreateMessageParams{
			Role:  msg.Role,
			Parts: msg.Parts,
		})
		if err != nil {
			return fmt.Errorf("copy message %s: %w", msg.ID, err)
		}

		// Stop after copying the target message.
		if msg.ID == upToMessageID {
			break
		}
	}

	return nil
}
