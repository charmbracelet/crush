package crush

import (
	"context"

	"github.com/charmbracelet/crush/internal/pubsub"
)

type (
	Event[T any]      = pubsub.Event[T]
	EventType         = pubsub.EventType
	Payload           = pubsub.Payload
	PayloadType       = pubsub.PayloadType
	Subscriber[T any] = pubsub.Subscriber[T]
	Publisher[T any]  = pubsub.Publisher[T]
)

const (
	EventCreated = pubsub.CreatedEvent
	EventUpdated = pubsub.UpdatedEvent
	EventDeleted = pubsub.DeletedEvent

	PayloadTypeLSPEvent               = pubsub.PayloadTypeLSPEvent
	PayloadTypeMCPEvent               = pubsub.PayloadTypeMCPEvent
	PayloadTypePermissionRequest      = pubsub.PayloadTypePermissionRequest
	PayloadTypePermissionNotification = pubsub.PayloadTypePermissionNotification
	PayloadTypeMessage                = pubsub.PayloadTypeMessage
	PayloadTypeSession                = pubsub.PayloadTypeSession
	PayloadTypeFile                   = pubsub.PayloadTypeFile
	PayloadTypeAgentEvent             = pubsub.PayloadTypeAgentEvent
	PayloadTypeSkillsEvent            = pubsub.PayloadTypeSkillsEvent
)

// SubscribeSessionMessages subscribes to message events for a specific
// session. The returned channel is closed when ctx is done. Events are
// typed as [Event[Message]]; use the helper methods on [Message] to
// inspect thinking text, tool calls, tool results, and regular content.
func SubscribeSessionMessages(ctx context.Context, app *App, sessionID string) <-chan Event[Message] {
	return app.SubscribeSessionMessages(ctx, sessionID)
}

// SubscribeSessionMessages subscribes to message events for a specific
// session. The returned channel is closed when ctx is done. Events are
// typed as [Event[Message]]; use the helper methods on [Message] to
// inspect thinking text, tool calls, tool results, and regular content.
func (a *App) SubscribeSessionMessages(ctx context.Context, sessionID string) <-chan Event[Message] {
	out := make(chan Event[Message], 8)
	go func() {
		defer close(out)
		ch := a.Messages.Subscribe(ctx)
		for ev := range ch {
			if ev.Payload.SessionID == sessionID {
				select {
				case out <- ev:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out
}
