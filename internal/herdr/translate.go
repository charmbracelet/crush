package herdr

import (
	"context"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/session"
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
		return PermissionRequested{
			ToolCallID:  e.Payload.ToolCallID,
			ToolName:    e.Payload.ToolName,
			Description: e.Payload.Description,
		}
	case pubsub.Event[permission.PermissionNotification]:
		return permissionNotification(e.Payload.ToolCallID, e.Payload.Granted, e.Payload.Denied)
	case pubsub.Event[question.Request]:
		var text string
		if len(e.Payload.Questions) > 0 {
			text = e.Payload.Questions[0].Text
		}
		return QuestionAsked{BatchID: e.Payload.ID, Text: truncateText(firstLine(text))}
	case pubsub.Event[question.Notification]:
		return QuestionResolved{BatchID: e.Payload.BatchID}
	case pubsub.Event[notify.Notification]:
		// Only re-authentication blocks the pane; every other
		// notification type is informational. AWS SSO is handled
		// transparently by its own dialog flow and never blocks.
		if e.Payload.Type == notify.TypeReAuthenticate {
			return AuthRequired{ProviderID: e.Payload.ProviderID}
		}
		return nil
	case pubsub.Event[session.Session]:
		return translateSession(e.Type, e.Payload.ID, e.Payload.Title)

	// Proto types (client/server mode). proto.Message carries no
	// IsSummaryMessage flag, so compaction is not detectable on the
	// wire; client/server mode simply never reports a summarizing
	// state.
	case pubsub.Event[proto.Message]:
		switch e.Payload.Role {
		case proto.Assistant:
			return AssistantMessage{SessionID: e.Payload.SessionID, Model: e.Payload.Model, Finished: e.Payload.IsFinished()}
		case proto.User:
			// Same exclusions as the domain mapping: only a
			// creation starts a run. Bang-mode shell records,
			// session-cleanup deletions and updates do not.
			if e.Type != pubsub.CreatedEvent || hasShellCommandPart(e.Payload.Parts) {
				return nil
			}
			return RunStarted{SessionID: e.Payload.SessionID}
		}
		return nil
	case pubsub.Event[proto.RunComplete]:
		return RunComplete{SessionID: e.Payload.SessionID}
	case pubsub.Event[proto.PermissionRequest]:
		return PermissionRequested{
			ToolCallID:  e.Payload.ToolCallID,
			ToolName:    e.Payload.ToolName,
			Description: e.Payload.Description,
		}
	case pubsub.Event[proto.PermissionNotification]:
		return permissionNotification(e.Payload.ToolCallID, e.Payload.Granted, e.Payload.Denied)
	case pubsub.Event[proto.QuestionRequest]:
		var text string
		if len(e.Payload.Questions) > 0 {
			text = e.Payload.Questions[0].Question
		}
		return QuestionAsked{BatchID: e.Payload.ID, Text: truncateText(firstLine(text))}
	case pubsub.Event[proto.QuestionNotification]:
		return QuestionResolved{BatchID: e.Payload.BatchID}
	case pubsub.Event[proto.AgentEvent]:
		// The server wraps notify.Notification into proto.AgentEvent
		// with the domain type string, so re_authenticate arrives
		// here in client/server mode. ProviderID does not cross the
		// wire, so the block message carries no provider name.
		if notify.Type(e.Payload.Type) == notify.TypeReAuthenticate {
			return AuthRequired{}
		}
		return nil
	case pubsub.Event[proto.Session]:
		return translateSession(e.Type, e.Payload.ID, e.Payload.Title)

	default:
		return nil
	}
}

// permissionNotification maps a permission notification to a herdr
// event. Only a granted or denied notification ends the wait: the
// permission service also publishes a flagless notification the
// moment a request *starts*, before publishing the request itself
// (permissionService.Request). Treating that marker as a resolution
// is unsafe because local mode consumes the request broker and the
// notification broker on separate goroutines, so the clear can land
// after the set and leave the pane reporting working while the
// prompt is still on screen.
func permissionNotification(toolCallID string, granted, denied bool) Event {
	if !granted && !denied {
		return nil
	}
	return PermissionResolved{ToolCallID: toolCallID}
}

// maxTextFieldLength is herdr's 80-rune cap on report text fields.
// It applies to every string crush sends except the notification
// body: the agent report's blocked-reason message, the notification
// title, and the pane presentation's title and token values.
const maxTextFieldLength = 80

// truncateText caps a text field at herdr's limit, keeping the cut
// rune-safe.
func truncateText(s string) string {
	return truncateRunes(s, maxTextFieldLength)
}

// truncateRunes caps s at limit runes, keeping the cut rune-safe. The
// parameter is limit rather than max so the body does not shadow the
// builtin.
func truncateRunes(s string, limit int) string {
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	return string(r[:limit])
}

// firstLine reduces free-form text to a single trimmed line. herdr's
// text fields are single-line, so anything past the first newline is
// dropped rather than sent as an embedded break.
func firstLine(s string) string {
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// translateMessage maps a domain message event to a herdr event.
// Creating a user message marks prompt submission, the real start of
// a turn.
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
		return AssistantMessage{SessionID: msg.SessionID, Model: msg.Model, Finished: msg.IsFinished()}
	}
	if msg.Role == message.User {
		// Only the creation of a user message starts a turn.
		// Bang-mode shell commands are persisted as user messages
		// (shell.PersistOutput) but start no agent run, and session
		// cleanup deletes user messages. Updates are excluded for
		// the same reason: re-arming runActive outside a real
		// submission produces no matching RunComplete and strands
		// the pane in working.
		if eventType != pubsub.CreatedEvent || len(msg.ShellCommands()) > 0 {
			return nil
		}
		return RunStarted{SessionID: msg.SessionID}
	}
	return nil
}

// translateSession maps a session event (domain or proto) to a
// presentation refresh. Deletions carry no presentation value: when
// the current session is deleted the UI switches away and SetSession
// clears the metadata instead.
func translateSession(eventType pubsub.EventType, id, title string) Event {
	if eventType == pubsub.DeletedEvent {
		return nil
	}
	return SessionUpdated{SessionID: id, Title: title}
}

// hasShellCommandPart reports whether a proto message carries a
// bang-mode shell command part. proto.Message has no ShellCommands
// helper like the domain type, so check the parts directly.
func hasShellCommandPart(parts []proto.ContentPart) bool {
	for _, p := range parts {
		if _, ok := p.(proto.ShellCommand); ok {
			return true
		}
	}
	return false
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
	Notifications         pubsub.Subscriber[notify.Notification]
	Sessions              pubsub.Subscriber[session.Session]
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
	go forward(ctx, c, func(subCtx context.Context) <-chan pubsub.Event[notify.Notification] {
		return src.Notifications.Subscribe(subCtx)
	})
	go forward(ctx, c, func(subCtx context.Context) <-chan pubsub.Event[session.Session] {
		return src.Sessions.Subscribe(subCtx)
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
