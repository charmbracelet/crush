package message

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
)

// ConversationSnapshot preserves exact message IDs, tool results, and summary cursor.
type ConversationSnapshot struct {
	Session  session.Session
	Messages []db.Message
}

// SnapshotConversation flushes streaming updates before reading persistent messages.
func SnapshotConversation(ctx context.Context, messages Service, sess session.Session) (json.RawMessage, error) {
	s, ok := messages.(*service)
	if !ok {
		return nil, fmt.Errorf("message store does not support rewind")
	}
	if err := s.FlushAll(ctx); err != nil {
		return nil, err
	}
	rows, err := s.q.ListMessagesBySession(ctx, sess.ID)
	if err != nil {
		return nil, err
	}
	return json.Marshal(ConversationSnapshot{Session: sess, Messages: rows})
}

// RestoreConversation replaces messages and their summary cursor in one SQL transaction.
// Usage cost and the session identity remain cumulative across branches.
func RestoreConversation(ctx context.Context, messages Service, conn *sql.DB, sessionID string, data json.RawMessage) error {
	s, ok := messages.(*service)
	if !ok {
		return fmt.Errorf("message store does not support rewind")
	}
	var snapshot ConversationSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return err
	}
	if snapshot.Session.ID != sessionID {
		return fmt.Errorf("conversation belongs to another session")
	}
	for _, row := range snapshot.Messages {
		if row.SessionID != sessionID {
			return fmt.Errorf("message belongs to another session")
		}
	}
	if err := s.FlushAll(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old, err := s.q.ListMessagesBySession(ctx, sessionID)
	if err != nil {
		return err
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "DELETE FROM messages WHERE session_id = ?", sessionID); err != nil {
		return err
	}
	for _, row := range snapshot.Messages {
		_, err = tx.ExecContext(ctx, `INSERT INTO messages (id, session_id, role, parts, model, created_at, updated_at, finished_at, provider, is_summary_message, prism_model_id, prism_model_name, prism_hypercredit_savings, prism_dollar_savings) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			row.ID, row.SessionID, row.Role, row.Parts, row.Model, row.CreatedAt, row.UpdatedAt, row.FinishedAt, row.Provider, row.IsSummaryMessage, row.PrismModelID, row.PrismModelName, row.PrismHypercreditSavings, row.PrismDollarSavings)
		if err != nil {
			return err
		}
	}
	todos, err := json.Marshal(snapshot.Session.Todos)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE sessions SET summary_message_id = ?, prompt_tokens = ?, completion_tokens = ?, todos = ? WHERE id = ?`,
		sql.NullString{String: snapshot.Session.SummaryMessageID, Valid: snapshot.Session.SummaryMessageID != ""}, snapshot.Session.PromptTokens, snapshot.Session.CompletionTokens, string(todos), sessionID)
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	for _, row := range old {
		delete(s.pending, row.ID)
		if msg, e := s.fromDBItem(row); e == nil {
			s.Publish(pubsub.DeletedEvent, msg)
		}
	}
	for _, row := range snapshot.Messages {
		delete(s.pending, row.ID)
		if msg, e := s.fromDBItem(row); e == nil {
			s.Publish(pubsub.CreatedEvent, msg)
		}
	}
	return nil
}
