package agent

import (
	"context"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/plugin"
)

// MessageSubscriberAdapter adapts message.Service to plugin.MessageSubscriber.
type MessageSubscriberAdapter struct {
	messages message.Service
}

// NewMessageSubscriberAdapter creates a new adapter for message events.
func NewMessageSubscriberAdapter(messages message.Service) *MessageSubscriberAdapter {
	return &MessageSubscriberAdapter{messages: messages}
}

// SubscribeMessages returns a channel that receives plugin message events.
func (a *MessageSubscriberAdapter) SubscribeMessages(ctx context.Context) <-chan plugin.MessageEvent {
	out := make(chan plugin.MessageEvent, 64)

	go func() {
		defer close(out)

		internalEvents := a.messages.Subscribe(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-internalEvents:
				if !ok {
					return
				}
				pluginEvent := a.convertEvent(event)
				select {
				case out <- pluginEvent:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return out
}

func (a *MessageSubscriberAdapter) convertEvent(event pubsub.Event[message.Message]) plugin.MessageEvent {
	var eventType plugin.MessageEventType
	switch event.Type {
	case pubsub.CreatedEvent:
		eventType = plugin.MessageCreated
	case pubsub.UpdatedEvent:
		eventType = plugin.MessageUpdated
	case pubsub.DeletedEvent:
		eventType = plugin.MessageDeleted
	default:
		eventType = plugin.MessageEventType(event.Type)
	}

	return plugin.MessageEvent{
		Type:    eventType,
		Message: a.convertMessage(event.Payload),
	}
}

func (a *MessageSubscriberAdapter) convertMessage(msg message.Message) plugin.Message {
	var role plugin.MessageRole
	switch msg.Role {
	case message.User:
		role = plugin.MessageRoleUser
	case message.Assistant:
		role = plugin.MessageRoleAssistant
	case message.System:
		role = plugin.MessageRoleSystem
	case message.Tool:
		role = plugin.MessageRoleTool
	default:
		role = plugin.MessageRole(msg.Role)
	}

	// Extract text content.
	content := msg.Content().Text

	// Convert tool calls.
	var toolCalls []plugin.ToolCallInfo
	for _, tc := range msg.ToolCalls() {
		toolCalls = append(toolCalls, plugin.ToolCallInfo{
			ID:       tc.ID,
			Name:     tc.Name,
			Input:    tc.Input,
			Finished: tc.Finished,
		})
	}

	// Convert tool results.
	var toolResults []plugin.ToolResultInfo
	for _, tr := range msg.ToolResults() {
		toolResults = append(toolResults, plugin.ToolResultInfo{
			ToolCallID: tr.ToolCallID,
			Name:       tr.Name,
			Content:    tr.Content,
			IsError:    tr.IsError,
		})
	}

	return plugin.Message{
		ID:          msg.ID,
		SessionID:   msg.SessionID,
		Role:        role,
		Content:     content,
		ToolCalls:   toolCalls,
		ToolResults: toolResults,
		CreatedAt:   msg.CreatedAt,
		UpdatedAt:   msg.UpdatedAt,
	}
}
