// Package fork provides conversation forking functionality.
// Forking creates a new session from a specific point in conversation history,
// optionally restoring the filesystem to that snapshot.
package fork

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
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
	// Step 1: Get the snapshot for the target message (if snapshots enabled).
	var targetSnapshot *checkpoint.Snapshot
	if s.checkpoints != nil && s.checkpoints.IsEnabled() {
		var err error
		targetSnapshot, err = s.checkpoints.GetSnapshotByMessage(ctx, params.MessageID)
		if err != nil && !errors.Is(err, checkpoint.ErrSnapshotNotFound) {
			return nil, fmt.Errorf("get snapshot: %w", err)
		}
		// If no snapshot exists for this message, that's okay - we just won't restore.
		// This can happen with archived/gc'd conversations.
		if errors.Is(err, checkpoint.ErrSnapshotNotFound) {
			slog.Warn("No snapshot found for fork message, filesystem will not be restored",
				"message_id", params.MessageID)
		}
	}

	// Step 2: Snapshot current state as a "stash" before making changes.
	// This allows us to restore if something goes wrong or if the user wants to undo.
	var stashSnapshot *checkpoint.Snapshot
	if s.checkpoints != nil && s.checkpoints.IsEnabled() {
		// Create a temporary snapshot of current state. We use a synthetic message ID
		// since this is a pre-fork stash, not tied to a specific message.
		stash, err := s.checkpoints.CreateSnapshot(ctx, params.SessionID, "pre-fork-stash", "Pre-fork filesystem state")
		if err != nil {
			slog.Warn("Failed to create pre-fork stash snapshot", "error", err)
			// Non-fatal - continue with fork even if stash fails.
		} else {
			stashSnapshot = stash
			_ = stashSnapshot // Stash is created for potential future undo functionality.
		}
	}

	// Step 3: Get source session to copy title if needed.
	sourceSession, err := s.sessions.Get(ctx, params.SessionID)
	if err != nil {
		return nil, fmt.Errorf("get source session: %w", err)
	}

	// Generate title if not provided.
	title := params.Title
	if title == "" {
		title = fmt.Sprintf("Fork of %s", sourceSession.Title)
	}

	// Step 4: Create new session.
	newSession, err := s.sessions.Create(ctx, title)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	// Step 5: Copy messages up to and including the specified message.
	idMapping, err := s.copyMessagesUpTo(ctx, params.SessionID, newSession.ID, params.MessageID)
	if err != nil {
		// Clean up on failure.
		_ = s.sessions.Delete(ctx, newSession.ID)
		return nil, fmt.Errorf("copy messages: %w", err)
	}

	// Step 6: Update session with SummaryMessageID if the source had one and it was copied.
	if sourceSession.SummaryMessageID != "" {
		if newSummaryID, ok := idMapping[sourceSession.SummaryMessageID]; ok {
			newSession.SummaryMessageID = newSummaryID
			newSession, err = s.sessions.Save(ctx, newSession)
			if err != nil {
				slog.Warn("Failed to update SummaryMessageID on forked session", "error", err)
			}
		}
	}

	// Step 7: Update session with fork reference.
	if targetSnapshot != nil {
		if err := s.queries.UpdateSessionForkedFrom(ctx, db.UpdateSessionForkedFromParams{
			ForkedFromSnapshotID: sql.NullString{String: targetSnapshot.ID, Valid: true},
			ID:                   newSession.ID,
		}); err != nil {
			slog.Warn("Failed to update session fork reference", "error", err)
		}
	}

	result := &ForkResult{
		NewSession:     newSession,
		SourceSnapshot: targetSnapshot,
		CreatedAt:      time.Now(),
	}

	// Step 8: Handle worktree creation or filesystem restore.
	if params.CreateWorktree && s.worktrees != nil && s.worktrees.IsEnabled() {
		// Create a worktree with the target snapshot state.
		snapshotID := ""
		if targetSnapshot != nil {
			snapshotID = targetSnapshot.ID
		}

		wt, err := s.worktrees.Create(ctx, newSession.ID, params.WorktreeName, snapshotID)
		if err != nil {
			slog.Warn("Failed to create worktree for fork", "error", err)
			// Non-fatal - session was still created.
		} else {
			result.Worktree = wt
		}
	} else if targetSnapshot != nil {
		// No worktree requested - restore the target snapshot to current directory.
		if err := s.checkpoints.RestoreSnapshot(ctx, targetSnapshot.ID, ""); err != nil {
			slog.Warn("Failed to restore snapshot for fork",
				"snapshot_id", targetSnapshot.ID,
				"error", err)
			// Non-fatal - session was still created, just filesystem wasn't restored.
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
// up to and including the specified message. Returns a map of old message IDs
// to new message IDs for updating references like SummaryMessageID.
func (s *service) copyMessagesUpTo(ctx context.Context, sourceSessionID, targetSessionID, upToMessageID string) (map[string]string, error) {
	msgs, err := s.messages.List(ctx, sourceSessionID)
	if err != nil {
		return nil, err
	}

	// Map old message IDs to new message IDs.
	idMapping := make(map[string]string)

	for _, msg := range msgs {
		// Create a copy of the message in the new session, preserving all fields.
		newMsg, err := s.messages.Create(ctx, targetSessionID, message.CreateMessageParams{
			Role:             msg.Role,
			Parts:            msg.Parts,
			Model:            msg.Model,
			Provider:         msg.Provider,
			IsSummaryMessage: msg.IsSummaryMessage,
		})
		if err != nil {
			return nil, fmt.Errorf("copy message %s: %w", msg.ID, err)
		}

		idMapping[msg.ID] = newMsg.ID

		// Stop after copying the target message.
		if msg.ID == upToMessageID {
			break
		}
	}

	return idMapping, nil
}
