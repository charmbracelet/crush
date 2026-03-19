package agent

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/plugin"
)

// SessionStoreAdapter adapts internal session/message services to plugin.SessionStore.
type SessionStoreAdapter struct {
	sessions session.Service
	messages message.Service
	q        *db.Queries
	conn     *sql.DB
}

// NewSessionStoreAdapter creates a new adapter.
func NewSessionStoreAdapter(sessions session.Service, messages message.Service, q *db.Queries, conn *sql.DB) *SessionStoreAdapter {
	return &SessionStoreAdapter{
		sessions: sessions,
		messages: messages,
		q:        q,
		conn:     conn,
	}
}

// GetSession returns a session by ID.
func (a *SessionStoreAdapter) GetSession(ctx context.Context, id string) (*plugin.SessionData, error) {
	sess, err := a.sessions.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return a.toPluginSession(sess), nil
}

// CreateSession creates a new session.
func (a *SessionStoreAdapter) CreateSession(ctx context.Context, title string) (*plugin.SessionData, error) {
	sess, err := a.sessions.Create(ctx, title)
	if err != nil {
		return nil, err
	}
	return a.toPluginSession(sess), nil
}

// ListSessionMessages returns all messages for a session.
func (a *SessionStoreAdapter) ListSessionMessages(ctx context.Context, sessionID string) ([]plugin.SessionMessage, error) {
	msgs, err := a.messages.List(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	result := make([]plugin.SessionMessage, len(msgs))
	for i, msg := range msgs {
		result[i] = a.toPluginMessage(msg)
	}
	return result, nil
}

// ExportSession exports a full session snapshot.
func (a *SessionStoreAdapter) ExportSession(ctx context.Context, sessionID string) (*plugin.SessionSnapshot, error) {
	sess, err := a.sessions.Get(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	msgs, err := a.messages.List(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to list messages: %w", err)
	}

	messages := make([]plugin.SessionMessage, len(msgs))
	for i, msg := range msgs {
		messages[i] = a.toPluginMessage(msg)
	}

	return &plugin.SessionSnapshot{
		Version:  plugin.CurrentSnapshotVersion,
		Session:  *a.toPluginSession(sess),
		Messages: messages,
	}, nil
}

// ImportSession imports a full session snapshot.
// If a session with the same ID already exists, its messages are replaced.
func (a *SessionStoreAdapter) ImportSession(ctx context.Context, snapshot plugin.SessionSnapshot) error {
	tx, err := a.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	qtx := a.q.WithTx(tx)

	// Delete existing session and messages if present.
	_ = qtx.DeleteSessionMessages(ctx, snapshot.Session.ID)
	_ = qtx.DeleteSession(ctx, snapshot.Session.ID)

	// Create the session.
	_, err = qtx.CreateSession(ctx, db.CreateSessionParams{
		ID:               snapshot.Session.ID,
		ParentSessionID:  sql.NullString{},
		Title:            snapshot.Session.Title,
		MessageCount:     snapshot.Session.MessageCount,
		PromptTokens:     snapshot.Session.PromptTokens,
		CompletionTokens: snapshot.Session.CompletionTokens,
		Cost:             snapshot.Session.Cost,
	})
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	// Update summary_message_id if set.
	if snapshot.Session.SummaryMessageID != "" {
		_, err = qtx.UpdateSession(ctx, db.UpdateSessionParams{
			ID:               snapshot.Session.ID,
			Title:            snapshot.Session.Title,
			PromptTokens:     snapshot.Session.PromptTokens,
			CompletionTokens: snapshot.Session.CompletionTokens,
			SummaryMessageID: sql.NullString{String: snapshot.Session.SummaryMessageID, Valid: true},
			Cost:             snapshot.Session.Cost,
		})
		if err != nil {
			return fmt.Errorf("failed to update session summary: %w", err)
		}
	}

	// Import messages in order.
	for _, msg := range snapshot.Messages {
		isSummary := int64(0)
		if msg.IsSummaryMessage {
			isSummary = 1
		}
		_, err = qtx.CreateMessage(ctx, db.CreateMessageParams{
			ID:               msg.ID,
			SessionID:        snapshot.Session.ID,
			Role:             msg.Role,
			Parts:            msg.Parts,
			Model:            sql.NullString{String: msg.Model, Valid: msg.Model != ""},
			Provider:         sql.NullString{String: msg.Provider, Valid: msg.Provider != ""},
			IsSummaryMessage: isSummary,
		})
		if err != nil {
			return fmt.Errorf("failed to import message %s: %w", msg.ID, err)
		}
	}

	return tx.Commit()
}

func (a *SessionStoreAdapter) toPluginSession(sess session.Session) *plugin.SessionData {
	return &plugin.SessionData{
		ID:               sess.ID,
		Title:            sess.Title,
		SummaryMessageID: sess.SummaryMessageID,
		MessageCount:     sess.MessageCount,
		PromptTokens:     sess.PromptTokens,
		CompletionTokens: sess.CompletionTokens,
		Cost:             sess.Cost,
		CreatedAt:        sess.CreatedAt,
		UpdatedAt:        sess.UpdatedAt,
	}
}

func (a *SessionStoreAdapter) toPluginMessage(msg message.Message) plugin.SessionMessage {
	// Re-serialize parts to JSON for the plugin boundary.
	// This preserves the full internal format.
	partsJSON := "[]"
	if len(msg.Parts) > 0 {
		if data, err := marshalPartsForExport(msg.Parts); err == nil {
			partsJSON = string(data)
		}
	}
	return plugin.SessionMessage{
		ID:               msg.ID,
		SessionID:        msg.SessionID,
		Role:             string(msg.Role),
		Parts:            partsJSON,
		Model:            msg.Model,
		Provider:         msg.Provider,
		IsSummaryMessage: msg.IsSummaryMessage,
		CreatedAt:        msg.CreatedAt,
		UpdatedAt:        msg.UpdatedAt,
	}
}

// marshalPartsForExport serializes message parts to JSON for export.
// This is a pass-through to the internal marshalParts but accessible from the adapter.
func marshalPartsForExport(parts []message.ContentPart) ([]byte, error) {
	return message.MarshalParts(parts)
}

// Ensure SessionStoreAdapter satisfies the interface at compile time.
var _ plugin.SessionStore = (*SessionStoreAdapter)(nil)
