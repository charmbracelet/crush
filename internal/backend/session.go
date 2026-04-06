package backend

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/session"
)

// CreateSession creates a new session in the given workspace.
func (b *Backend) CreateSession(ctx context.Context, workspaceID, title string) (session.Session, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return session.Session{}, err
	}

	return ws.Sessions.Create(ctx, title)
}

// GetSession retrieves a session by workspace and session ID.
func (b *Backend) GetSession(ctx context.Context, workspaceID, sessionID string) (session.Session, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return session.Session{}, err
	}

	return ws.Sessions.Get(ctx, sessionID)
}

// ListSessions returns all sessions in the given workspace.
func (b *Backend) ListSessions(ctx context.Context, workspaceID string) ([]session.Session, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	return ws.Sessions.List(ctx)
}

// GetAgentSession returns session metadata with the agent's busy
// status.
func (b *Backend) GetAgentSession(ctx context.Context, workspaceID, sessionID string) (proto.AgentSession, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return proto.AgentSession{}, err
	}

	se, err := ws.Sessions.Get(ctx, sessionID)
	if err != nil {
		return proto.AgentSession{}, err
	}

	var isSessionBusy bool
	if ws.AgentCoordinator != nil {
		isSessionBusy = ws.AgentCoordinator.IsSessionBusy(sessionID)
	}

	return proto.AgentSession{
		Session: proto.Session{
			ID:    se.ID,
			Title: se.Title,
		},
		IsBusy: isSessionBusy,
	}, nil
}

// ListSessionMessages returns all messages for a session.
func (b *Backend) ListSessionMessages(ctx context.Context, workspaceID, sessionID string) ([]message.Message, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	return ws.Messages.List(ctx, sessionID)
}

// ListSessionHistory returns the history items for a session.
func (b *Backend) ListSessionHistory(ctx context.Context, workspaceID, sessionID string) (any, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	return ws.History.ListBySession(ctx, sessionID)
}

// SaveSession updates a session in the given workspace.
func (b *Backend) SaveSession(ctx context.Context, workspaceID string, sess session.Session) (session.Session, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return session.Session{}, err
	}

	return ws.Sessions.Save(ctx, sess)
}

// DeleteSession deletes a session from the given workspace.
func (b *Backend) DeleteSession(ctx context.Context, workspaceID, sessionID string) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}

	return ws.Sessions.Delete(ctx, sessionID)
}

// ListUserMessages returns user-role messages for a session.
func (b *Backend) ListUserMessages(ctx context.Context, workspaceID, sessionID string) ([]message.Message, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	return ws.Messages.ListUserMessages(ctx, sessionID)
}

// ListAllUserMessages returns all user-role messages across sessions.
func (b *Backend) ListAllUserMessages(ctx context.Context, workspaceID string) ([]message.Message, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	return ws.Messages.ListAllUserMessages(ctx)
}

// UndoLastMessage rolls the session back by one user message.
func (b *Backend) UndoLastMessage(ctx context.Context, workspaceID, sessionID string) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}
	return backendUndo(ctx, ws, sessionID)
}

// RedoMessage moves the revert marker forward by one user message or clears it.
func (b *Backend) RedoMessage(ctx context.Context, workspaceID, sessionID string) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}
	return backendRedo(ctx, ws, sessionID)
}

// CleanupRevert permanently discards the hidden messages and file records.
func (b *Backend) CleanupRevert(ctx context.Context, workspaceID, sessionID string) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}
	return backendCleanupRevert(ctx, ws, sessionID)
}

// backendUndo performs the undo logic directly against app services to avoid
// an import cycle between backend and workspace.
func backendUndo(ctx context.Context, ws *Workspace, sessionID string) error {
	sess, err := ws.Sessions.Get(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("getting session: %w", err)
	}
	var targetMsg message.Message
	if sess.RevertMessageID == "" {
		msgs, err := ws.Messages.ListUserMessages(ctx, sessionID)
		if err != nil {
			return fmt.Errorf("listing user messages: %w", err)
		}
		if len(msgs) == 0 {
			return errors.New("nothing to undo")
		}
		targetMsg = msgs[0]
	} else {
		prev, err := ws.Messages.FindPreviousUserMessage(ctx, sessionID, sess.RevertMessageID)
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("nothing more to undo")
		}
		if err != nil {
			return fmt.Errorf("finding previous user message: %w", err)
		}
		targetMsg = prev
	}
	if err := ws.History.RestoreToTimestamp(ctx, sessionID, targetMsg.CreatedAt); err != nil {
		return fmt.Errorf("restoring files: %w", err)
	}
	return ws.Sessions.SetRevert(ctx, sessionID, targetMsg.ID)
}

// backendRedo performs the redo logic directly against app services.
func backendRedo(ctx context.Context, ws *Workspace, sessionID string) error {
	sess, err := ws.Sessions.Get(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("getting session: %w", err)
	}
	if sess.RevertMessageID == "" {
		return errors.New("nothing to redo")
	}
	next, err := ws.Messages.FindNextUserMessage(ctx, sessionID, sess.RevertMessageID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("finding next user message: %w", err)
	}
	if errors.Is(err, sql.ErrNoRows) {
		// No later user message — restore to head state.
		if err := ws.History.RestoreToLatest(ctx, sessionID); err != nil {
			return fmt.Errorf("restoring files to latest: %w", err)
		}
		return ws.Sessions.ClearRevert(ctx, sessionID)
	}
	if err := ws.History.RestoreToTimestamp(ctx, sessionID, next.CreatedAt); err != nil {
		return fmt.Errorf("restoring files: %w", err)
	}
	return ws.Sessions.SetRevert(ctx, sessionID, next.ID)
}

// backendCleanupRevert permanently discards the undone messages and file records.
func backendCleanupRevert(ctx context.Context, ws *Workspace, sessionID string) error {
	sess, err := ws.Sessions.Get(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("getting session: %w", err)
	}
	if sess.RevertMessageID == "" {
		return nil
	}
	// Use the first undone message as the boundary rather than the revert
	// message itself to avoid same-second timestamp collisions.
	nextMsg, err := ws.Messages.FindNextUserMessage(ctx, sessionID, sess.RevertMessageID)
	if errors.Is(err, sql.ErrNoRows) {
		return ws.Sessions.ClearRevert(ctx, sessionID)
	}
	if err != nil {
		return fmt.Errorf("finding first undone message: %w", err)
	}
	if err := ws.Messages.DeleteMessagesAfterTimestamp(ctx, sessionID, nextMsg.CreatedAt); err != nil {
		return fmt.Errorf("deleting messages: %w", err)
	}
	if err := ws.History.CleanupAfterTimestamp(ctx, sessionID, nextMsg.CreatedAt); err != nil {
		return fmt.Errorf("cleaning up file versions: %w", err)
	}
	return ws.Sessions.ClearRevert(ctx, sessionID)
}
