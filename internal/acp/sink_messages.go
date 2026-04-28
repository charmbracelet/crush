package acp

import (
	"log/slog"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/coder/acp-go-sdk"
)

// HandleMessage translates a Crush message event to ACP session updates.
func (s *Sink) HandleMessage(event pubsub.Event[message.Message]) {
	msg := event.Payload

	// Only handle messages for our session.
	if msg.SessionID != s.sessionID {
		return
	}

	for _, part := range msg.Parts {
		update := s.translatePart(msg.ID, msg.Role, part)
		if update == nil {
			continue
		}

		if err := s.conn.SessionUpdate(s.ctx, acp.SessionNotification{
			SessionId: acp.SessionId(s.sessionID),
			Update:    *update,
		}); err != nil {
			slog.Error("Failed to send session update", "error", err)
		}
	}
}

// translatePart converts a message part to an ACP session update.
func (s *Sink) translatePart(msgID string, role message.MessageRole, part message.ContentPart) *acp.SessionUpdate {
	switch p := part.(type) {
	case message.TextContent:
		return s.translateText(msgID, role, p)

	case message.ReasoningContent:
		return s.translateReasoning(msgID, p)

	case message.ToolCall:
		return s.translateToolCall(p)

	case message.ToolResult:
		return s.translateToolResult(p)

	case message.Finish:
		// Reset offsets on message finish.
		delete(s.textOffsets, msgID)
		delete(s.reasoningOffsets, msgID)
		return nil

	default:
		return nil
	}
}

func (s *Sink) translateText(msgID string, role message.MessageRole, text message.TextContent) *acp.SessionUpdate {
	// Skip user messages - the client already knows what it sent via the
	// prompt request.
	if role != message.Assistant {
		return nil
	}

	offset := s.textOffsets[msgID]
	if len(text.Text) <= offset {
		return nil
	}

	delta := text.Text[offset:]
	s.textOffsets[msgID] = len(text.Text)

	if delta == "" {
		return nil
	}

	update := acp.UpdateAgentMessageText(delta)
	return &update
}

func (s *Sink) translateReasoning(msgID string, reasoning message.ReasoningContent) *acp.SessionUpdate {
	offset := s.reasoningOffsets[msgID]
	if len(reasoning.Thinking) <= offset {
		return nil
	}

	delta := reasoning.Thinking[offset:]
	s.reasoningOffsets[msgID] = len(reasoning.Thinking)

	if delta == "" {
		return nil
	}

	update := acp.UpdateAgentThoughtText(delta)
	return &update
}
