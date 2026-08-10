package herdr

import (
	"context"
	"time"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/question"
)

// Translate converts a pub/sub event (domain or proto) into a herdr
// Event. Returns nil for event types herdr doesn't care about. This
// is the single translation point for all integration modes.
func Translate(ev any) Event {
	switch e := ev.(type) {
	// Domain types (TUI / local headless).
	case pubsub.Event[message.Message]:
		return translateMessage(e.Type, e.Payload)
	case pubsub.Event[notify.RunComplete]:
		return RunComplete{SessionID: e.Payload.SessionID}
	case pubsub.Event[permission.PermissionRequest]:
		return PermissionRequested{}
	case pubsub.Event[permission.PermissionNotification]:
		return PermissionResolved{}
	case pubsub.Event[question.Request]:
		var text string
		if len(e.Payload.Questions) > 0 {
			text = e.Payload.Questions[0].Text
		}
		return QuestionAsked{Text: truncateBlockMessage(text)}
	case pubsub.Event[question.Notification]:
		return QuestionResolved{}

	// Proto types (client/server mode). proto.Message carries no
	// IsSummaryMessage flag, so compaction is not detectable on the
	// wire; client/server mode simply never reports a summarizing
	// state.
	case pubsub.Event[proto.Message]:
		if e.Payload.Role == proto.Assistant {
			return AssistantMessage{SessionID: e.Payload.SessionID}
		}
		return nil
	case pubsub.Event[proto.RunComplete]:
		return RunComplete{SessionID: e.Payload.SessionID}
	case pubsub.Event[proto.PermissionRequest]:
		return PermissionRequested{}
	case pubsub.Event[proto.PermissionNotification]:
		return PermissionResolved{}
	case pubsub.Event[proto.QuestionRequest]:
		var text string
		if len(e.Payload.Questions) > 0 {
			text = e.Payload.Questions[0].Question
		}
		return QuestionAsked{Text: truncateBlockMessage(text)}
	case pubsub.Event[proto.QuestionNotification]:
		return QuestionResolved{}

	default:
		return nil
	}
}

// maxBlockMessageLength is herdr's 80-character cap on report text
// fields. Block messages must stay under it.
const maxBlockMessageLength = 80

// truncateBlockMessage caps a blocked-reason message at herdr's
// text-field limit, keeping the cut rune-safe.
func truncateBlockMessage(s string) string {
	r := []rune(s)
	if len(r) <= maxBlockMessageLength {
		return s
	}
	return string(r[:maxBlockMessageLength])
}

// translateMessage maps a domain message event to a herdr event.
// Compaction never publishes a RunComplete, so the summary message's
// lifecycle doubles as the compaction signal: an unfinished summary
// message means compaction is running, a finished one means it
// completed or errored (sessionAgent.Summarize calls AddFinish on
// both paths), and a deleted one means the user cancelled (the
// cancel path removes the summary message).
func translateMessage(eventType pubsub.EventType, msg message.Message) Event {
	if msg.IsSummaryMessage {
		switch {
		case eventType == pubsub.DeletedEvent:
			return SummarizeFinished{SessionID: msg.SessionID}
		case msg.IsFinished():
			return SummarizeFinished{SessionID: msg.SessionID}
		default:
			return SummarizeStarted{SessionID: msg.SessionID}
		}
	}
	if msg.Role == message.Assistant {
		return AssistantMessage{SessionID: msg.SessionID}
	}
	return nil
}

// permNotificationSubscriber is the subset of the permission service
// needed by BridgeLocal to subscribe to permission notifications.
type permNotificationSubscriber interface {
	SubscribeNotifications(context.Context) <-chan pubsub.Event[permission.PermissionNotification]
}

// questionNotificationSubscriber is the subset of the question
// service needed by BridgeLocal to subscribe to resolution
// notifications.
type questionNotificationSubscriber interface {
	SubscribeNotifications(context.Context) <-chan pubsub.Event[question.Notification]
}

// BridgeSources groups the pub/sub sources that BridgeLocal subscribes
// to. Adding a new event type means adding a field here rather than
// growing the function signature.
type BridgeSources struct {
	PermRequests          pubsub.Subscriber[permission.PermissionRequest]
	PermNotifications     permNotificationSubscriber
	RunCompletions        pubsub.Subscriber[notify.RunComplete]
	Messages              pubsub.Subscriber[message.Message]
	Questions             pubsub.Subscriber[question.Request]
	QuestionNotifications questionNotificationSubscriber
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
	go forward(ctx, c, func(subCtx context.Context) <-chan pubsub.Event[question.Request] {
		return src.Questions.Subscribe(subCtx)
	})
	go forward(ctx, c, func(subCtx context.Context) <-chan pubsub.Event[question.Notification] {
		return src.QuestionNotifications.SubscribeNotifications(subCtx)
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
