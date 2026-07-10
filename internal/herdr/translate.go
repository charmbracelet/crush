package herdr

import (
	"context"
	"time"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
)

// Translate converts a pub/sub event (domain or proto) into a herdr
// Event. Returns nil for event types herdr doesn't care about. This
// is the single translation point for all integration modes.
func Translate(ev any) Event {
	switch e := ev.(type) {
	// Domain types (TUI / local headless).
	case pubsub.Event[message.Message]:
		return translateMessage(
			e.Payload.Role == message.Assistant,
			e.Payload.SessionID,
			e.Payload.IsSummaryMessage,
		)
	case pubsub.Event[notify.RunComplete]:
		return RunComplete{SessionID: e.Payload.SessionID}
	case pubsub.Event[permission.PermissionRequest]:
		return PermissionRequested{}
	case pubsub.Event[permission.PermissionNotification]:
		return PermissionResolved{}

	// Proto types (client/server mode).
	case pubsub.Event[proto.Message]:
		return translateMessage(
			e.Payload.Role == proto.Assistant,
			e.Payload.SessionID,
			false,
		)
	case pubsub.Event[proto.RunComplete]:
		return RunComplete{SessionID: e.Payload.SessionID}
	case pubsub.Event[proto.PermissionRequest]:
		return PermissionRequested{}
	case pubsub.Event[proto.PermissionNotification]:
		return PermissionResolved{}
	case pubsub.Event[proto.AgentEvent]:
		if e.Payload.Type == proto.AgentEventTypeSummarize && !e.Payload.Done {
			return Summarizing{}
		}
		return nil

	default:
		return nil
	}
}

// translateMessage is the shared message-mapping logic for both domain
// and proto message types.
func translateMessage(isAssistant bool, sessionID string, isSummary bool) Event {
	if !isAssistant {
		return nil
	}
	if isSummary {
		return Summarizing{}
	}
	return AssistantMessage{SessionID: sessionID}
}

// permNotificationSubscriber is the subset of the permission service
// needed by BridgeLocal to subscribe to permission notifications.
type permNotificationSubscriber interface {
	SubscribeNotifications(context.Context) <-chan pubsub.Event[permission.PermissionNotification]
}

// BridgeSources groups the pub/sub sources that BridgeLocal subscribes
// to. Adding a new event type means adding a field here rather than
// growing the function signature.
type BridgeSources struct {
	PermRequests      pubsub.Subscriber[permission.PermissionRequest]
	PermNotifications permNotificationSubscriber
	RunCompletions    pubsub.Subscriber[notify.RunComplete]
	Messages          pubsub.Subscriber[message.Message]
}

// BridgeLocal subscribes to local pub/sub brokers and forwards
// translated events to the client. Used in TUI and local headless
// modes where the agent runs in-process. Cancelling ctx stops the
// bridge goroutines.
//
// The spawned goroutines are best-effort and may briefly outlive
// Client.Close(). This is safe: HandleEvent is nil-safe, and the
// unixSender drops messages on a full buffer rather than blocking.
//
// Each goroutine uses a resilient subscription loop that re-subscribes
// if the channel closes unexpectedly, ensuring the bridge survives
// transient pub/sub broker resets.
func BridgeLocal(ctx context.Context, c *Client, src BridgeSources) {
	if c == nil {
		return
	}
	go forward(ctx, c, func(subCtx context.Context) <-chan pubsub.Event[permission.PermissionRequest] {
		return src.PermRequests.Subscribe(subCtx)
	})
	go forward(ctx, c, func(subCtx context.Context) <-chan pubsub.Event[permission.PermissionNotification] {
		return src.PermNotifications.SubscribeNotifications(subCtx)
	})
	go forward(ctx, c, func(subCtx context.Context) <-chan pubsub.Event[notify.RunComplete] {
		return src.RunCompletions.Subscribe(subCtx)
	})
	go forward(ctx, c, func(subCtx context.Context) <-chan pubsub.Event[message.Message] {
		return src.Messages.Subscribe(subCtx)
	})
}

// forward reads from a pub/sub channel and forwards translated
// events to the herdr client. If the channel closes (e.g., due to
// broker reset), it re-subscribes after a brief delay. Runs until ctx
// is cancelled.
func forward[T any](ctx context.Context, c *Client, subscribe func(context.Context) <-chan pubsub.Event[T]) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		subCtx, cancel := context.WithCancel(ctx)
		ch := subscribe(subCtx)

	inner:
		for {
			select {
			case <-ctx.Done():
				cancel()
				return
			case ev, ok := <-ch:
				if !ok {
					// Channel closed — broker may have reset.
					// Cancel the sub-context and re-subscribe.
					cancel()
					time.Sleep(100 * time.Millisecond)
					break inner
				}
				if hev := Translate(ev); hev != nil {
					c.HandleEvent(hev)
				}
			}
		}
	}
}
