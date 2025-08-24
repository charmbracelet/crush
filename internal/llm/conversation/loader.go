package conversation

import (
	"context"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
)

func LoadFromSession(
	ctx context.Context,
	session session.Session,
	msgService message.Service,
) (Conversation, error) {
	messages, err := msgService.List(ctx, session.ID)
	if err != nil {
		return Conversation{}, err
	}

	var turns []message.Message
	for _, msg := range messages {
		afterSummary := false
		role := message.MessageRole(msg.Role)

		if session.SummaryMessageID == "" {
			afterSummary = true
		} else if msg.ID == session.SummaryMessageID {
			afterSummary = true
			role = message.User
		}

		if afterSummary {
			turns = append(turns, message.Message{
				Role:  message.MessageRole(role),
				Parts: msg.Parts,
			})
		}
	}

	return New(turns...), nil
}
