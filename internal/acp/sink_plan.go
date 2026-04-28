package acp

import (
	"log/slog"

	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/coder/acp-go-sdk"
)

// HandleSession translates session updates to ACP plan updates.
func (s *Sink) HandleSession(event pubsub.Event[session.Session]) {
	sess := event.Payload

	// Only handle updates for our session.
	if sess.ID != s.sessionID {
		return
	}

	// Only handle update events (not created/deleted).
	if event.Type != pubsub.UpdatedEvent {
		return
	}

	// Convert todos to plan entries.
	entries := make([]acp.PlanEntry, len(sess.Todos))
	for i, todo := range sess.Todos {
		entries[i] = acp.PlanEntry{
			Content:  todo.Content,
			Status:   acp.PlanEntryStatus(todo.Status),
			Priority: acp.PlanEntryPriorityMedium,
		}
		if todo.ActiveForm != "" {
			entries[i].Meta = map[string]string{"active_form": todo.ActiveForm}
		}
	}

	update := acp.UpdatePlan(entries...)
	if err := s.conn.SessionUpdate(s.ctx, acp.SessionNotification{
		SessionId: acp.SessionId(s.sessionID),
		Update:    update,
	}); err != nil {
		slog.Error("Failed to send plan update", "error", err)
	}
}
