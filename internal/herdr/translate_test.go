package herdr

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/assert"
)

// Domain type translation.

func TestTranslateDomainAssistantMessage(t *testing.T) {
	t.Parallel()
	ev := pubsub.Event[message.Message]{
		Payload: message.Message{Role: message.Assistant, SessionID: "s1", Model: "model-1"},
	}
	assert.Equal(t, AssistantMessage{SessionID: "s1", Model: "model-1"}, Translate(ev))
}

func TestTranslateDomainFinishedAssistantMessage(t *testing.T) {
	t.Parallel()
	// A finished assistant message is the turn's terminal snapshot
	// (ESC-cancel coalesces one with FinishReasonCanceled); it must
	// map with Finished set so the client does not treat it as
	// ongoing activity.
	ev := pubsub.Event[message.Message]{
		Type: pubsub.UpdatedEvent,
		Payload: message.Message{
			Role:      message.Assistant,
			SessionID: "s1",
			Model:     "model-1",
			Parts: []message.ContentPart{
				message.Finish{Reason: message.FinishReasonCanceled},
			},
		},
	}
	assert.Equal(t, AssistantMessage{SessionID: "s1", Model: "model-1", Finished: true}, Translate(ev))
}

func TestTranslateDomainSummaryMessageStarted(t *testing.T) {
	t.Parallel()
	// An unfinished summary message (created or mid-stream update)
	// means compaction is running.
	for _, eventType := range []pubsub.EventType{pubsub.CreatedEvent, pubsub.UpdatedEvent} {
		ev := pubsub.Event[message.Message]{
			Type: eventType,
			Payload: message.Message{
				Role:             message.Assistant,
				SessionID:        "s1",
				IsSummaryMessage: true,
			},
		}
		assert.Equal(t, SummarizeStarted{SessionID: "s1"}, Translate(ev))
	}
}

func TestTranslateDomainSummaryMessageFinished(t *testing.T) {
	t.Parallel()
	// Success and error both AddFinish on the summary message.
	ev := pubsub.Event[message.Message]{
		Type: pubsub.UpdatedEvent,
		Payload: message.Message{
			Role:             message.Assistant,
			SessionID:        "s1",
			IsSummaryMessage: true,
			Parts: []message.ContentPart{
				message.Finish{Reason: message.FinishReasonEndTurn},
			},
		},
	}
	assert.Equal(t, SummarizeFinished{SessionID: "s1"}, Translate(ev))
}

func TestTranslateDomainSummaryMessageDeleted(t *testing.T) {
	t.Parallel()
	// The cancel path deletes the summary message, so a DeletedEvent
	// for one also means compaction is over.
	ev := pubsub.Event[message.Message]{
		Type: pubsub.DeletedEvent,
		Payload: message.Message{
			Role:             message.Assistant,
			SessionID:        "s1",
			IsSummaryMessage: true,
		},
	}
	assert.Equal(t, SummarizeFinished{SessionID: "s1"}, Translate(ev))
}

func TestTranslateDomainNonAssistantIgnored(t *testing.T) {
	t.Parallel()
	ev := pubsub.Event[message.Message]{
		Payload: message.Message{Role: message.System},
	}
	assert.Nil(t, Translate(ev))
}

func TestTranslateDomainUserMessage(t *testing.T) {
	t.Parallel()
	// A user message marks prompt submission, the real start of a
	// turn.
	ev := pubsub.Event[message.Message]{
		Type: pubsub.CreatedEvent,
		Payload: message.Message{
			Role:      message.User,
			SessionID: "s1",
			Parts:     []message.ContentPart{message.TextContent{Text: "hi"}},
		},
	}
	assert.Equal(t, RunStarted{SessionID: "s1"}, Translate(ev))
}

func TestTranslateDomainUserMessageShellCommandIgnored(t *testing.T) {
	t.Parallel()
	// Bang-mode shell commands are persisted as user messages
	// (shell.PersistOutput) but start no agent run; mapping them
	// would leave the pane stuck in working.
	ev := pubsub.Event[message.Message]{
		Type: pubsub.CreatedEvent,
		Payload: message.Message{
			Role:      message.User,
			SessionID: "s1",
			Parts: []message.ContentPart{
				message.ShellCommand{Command: "ls", Output: "file.go", ExitCode: 0},
			},
		},
	}
	assert.Nil(t, Translate(ev))
}

func TestTranslateDomainUserMessageDeletedIgnored(t *testing.T) {
	t.Parallel()
	// Session cleanup deletes user messages; a deletion must not
	// report working.
	ev := pubsub.Event[message.Message]{
		Type:    pubsub.DeletedEvent,
		Payload: message.Message{Role: message.User, SessionID: "s1"},
	}
	assert.Nil(t, Translate(ev))
}

func TestTranslateDomainUserMessageUpdatedIgnored(t *testing.T) {
	t.Parallel()
	// Only a creation is a prompt submission. message.Service.Update
	// publishes an UpdatedEvent for whatever message it is handed, so
	// mapping updates would let a future edit-the-prompt feature
	// re-arm runActive with no matching RunComplete.
	ev := pubsub.Event[message.Message]{
		Type: pubsub.UpdatedEvent,
		Payload: message.Message{
			Role:      message.User,
			SessionID: "s1",
			Parts:     []message.ContentPart{message.TextContent{Text: "hi"}},
		},
	}
	assert.Nil(t, Translate(ev))
}

func TestTranslateDomainRunComplete(t *testing.T) {
	t.Parallel()
	ev := pubsub.Event[notify.RunComplete]{
		Payload: notify.RunComplete{SessionID: "s1"},
	}
	assert.Equal(t, RunComplete{SessionID: "s1"}, Translate(ev))
}

func TestTranslateDomainPermissionRequest(t *testing.T) {
	t.Parallel()
	ev := pubsub.Event[permission.PermissionRequest]{
		Payload: permission.PermissionRequest{
			ToolName:    "bash",
			ToolCallID:  "tc-1",
			Description: "Execute command: ls",
		},
	}
	assert.Equal(t, PermissionRequested{
		ToolCallID:  "tc-1",
		ToolName:    "bash",
		Description: "Execute command: ls",
	}, Translate(ev))
}

func TestTranslateDomainPermissionNotification(t *testing.T) {
	t.Parallel()
	granted := pubsub.Event[permission.PermissionNotification]{
		Payload: permission.PermissionNotification{ToolCallID: "tc-1", Granted: true},
	}
	assert.Equal(t, PermissionResolved{ToolCallID: "tc-1"}, Translate(granted))

	denied := pubsub.Event[permission.PermissionNotification]{
		Payload: permission.PermissionNotification{ToolCallID: "tc-1", Denied: true},
	}
	assert.Equal(t, PermissionResolved{ToolCallID: "tc-1"}, Translate(denied))

	// The permission service publishes a flagless notification when
	// a request starts, before publishing the request itself. It is
	// not a resolution and must not unblock the pane.
	marker := pubsub.Event[permission.PermissionNotification]{
		Payload: permission.PermissionNotification{ToolCallID: "tc-1"},
	}
	assert.Nil(t, Translate(marker))
}

func TestTranslateDomainQuestionRequest(t *testing.T) {
	t.Parallel()
	ev := pubsub.Event[question.Request]{
		Payload: question.Request{
			ID:        "b1",
			SessionID: "s1",
			Questions: []question.Question{
				{Text: "Which file should I edit?"},
				{Text: "And why?"},
			},
		},
	}
	// The blocked message is the first question's text.
	want := QuestionAsked{BatchID: "b1", Text: "Which file should I edit?"}
	assert.Equal(t, want, Translate(ev))
}

func TestTranslateDomainQuestionRequestEmptyQuestions(t *testing.T) {
	t.Parallel()
	// A request without questions still blocks, just without a
	// message.
	ev := pubsub.Event[question.Request]{
		Payload: question.Request{SessionID: "s1"},
	}
	assert.Equal(t, QuestionAsked{}, Translate(ev))
}

func TestTranslateDomainQuestionRequestTruncatesMessage(t *testing.T) {
	t.Parallel()
	// herdr caps text fields at 80 characters; the cut must be
	// rune-safe.
	ev := pubsub.Event[question.Request]{
		Payload: question.Request{
			Questions: []question.Question{
				{Text: strings.Repeat("界", 100)},
			},
		},
	}
	got := Translate(ev)
	want := QuestionAsked{Text: strings.Repeat("界", maxTextFieldLength)}
	assert.Equal(t, want, got)
}

func TestTranslateDomainQuestionRequestFirstLinesMessage(t *testing.T) {
	t.Parallel()
	// Question text is model-generated free-form; herdr's text
	// fields are single-line, so anything past the first newline
	// is dropped and the remainder trimmed.
	ev := pubsub.Event[question.Request]{
		Payload: question.Request{
			ID: "b1",
			Questions: []question.Question{
				{Text: "  Which file should I edit?\nAnd why?\r\nMore detail"},
			},
		},
	}
	want := QuestionAsked{BatchID: "b1", Text: "Which file should I edit?"}
	assert.Equal(t, want, Translate(ev))
}

func TestTranslateDomainQuestionRequestFirstLineStillTruncated(t *testing.T) {
	t.Parallel()
	// First-lining runs before the cap, so a long first line is
	// still cut at 80 runes.
	ev := pubsub.Event[question.Request]{
		Payload: question.Request{
			Questions: []question.Question{
				{Text: strings.Repeat("界", 100) + "\nignored"},
			},
		},
	}
	want := QuestionAsked{Text: strings.Repeat("界", maxTextFieldLength)}
	assert.Equal(t, want, Translate(ev))
}

func TestTranslateDomainQuestionNotification(t *testing.T) {
	t.Parallel()
	ev := pubsub.Event[question.Notification]{
		Payload: question.Notification{BatchID: "b1"},
	}
	assert.Equal(t, QuestionResolved{BatchID: "b1"}, Translate(ev))
}

func TestTranslateDomainReAuthenticateNotification(t *testing.T) {
	t.Parallel()
	ev := pubsub.Event[notify.Notification]{
		Payload: notify.Notification{
			Type:       notify.TypeReAuthenticate,
			ProviderID: "hyper",
		},
	}
	assert.Equal(t, AuthRequired{ProviderID: "hyper"}, Translate(ev))
}

func TestTranslateDomainOtherNotificationsIgnored(t *testing.T) {
	t.Parallel()
	// Only re-authentication blocks the pane; the remaining
	// notification types are informational or own their dialog
	// flows (AWS SSO).
	for _, typ := range []notify.Type{
		notify.TypeAgentFinished,
		notify.TypeAgentError,
		notify.TypeAWSSSOAuth,
		notify.TypeAWSSSOAuthResult,
	} {
		ev := pubsub.Event[notify.Notification]{
			Payload: notify.Notification{Type: typ},
		}
		assert.Nil(t, Translate(ev))
	}
}

// Proto type translation.

func TestTranslateProtoAssistantMessage(t *testing.T) {
	t.Parallel()
	ev := pubsub.Event[proto.Message]{
		Payload: proto.Message{Role: proto.Assistant, SessionID: "s1", Model: "model-1"},
	}
	assert.Equal(t, AssistantMessage{SessionID: "s1", Model: "model-1"}, Translate(ev))
}

func TestTranslateProtoFinishedAssistantMessage(t *testing.T) {
	t.Parallel()
	ev := pubsub.Event[proto.Message]{
		Payload: proto.Message{
			Role:      proto.Assistant,
			SessionID: "s1",
			Model:     "model-1",
			Parts: []proto.ContentPart{
				proto.Finish{Reason: proto.FinishReasonCanceled},
			},
		},
	}
	assert.Equal(t, AssistantMessage{SessionID: "s1", Model: "model-1", Finished: true}, Translate(ev))
}

func TestTranslateProtoNonAssistantIgnored(t *testing.T) {
	t.Parallel()
	ev := pubsub.Event[proto.Message]{
		Payload: proto.Message{Role: proto.System, SessionID: "s1"},
	}
	assert.Nil(t, Translate(ev))
}

func TestTranslateProtoUserMessage(t *testing.T) {
	t.Parallel()
	ev := pubsub.Event[proto.Message]{
		Type: pubsub.CreatedEvent,
		Payload: proto.Message{
			Role:      proto.User,
			SessionID: "s1",
			Parts:     []proto.ContentPart{proto.TextContent{Text: "hi"}},
		},
	}
	assert.Equal(t, RunStarted{SessionID: "s1"}, Translate(ev))
}

func TestTranslateProtoUserMessageShellCommandIgnored(t *testing.T) {
	t.Parallel()
	// Bang-mode shell records cross the wire with their
	// ShellCommand part; they start no run.
	ev := pubsub.Event[proto.Message]{
		Type: pubsub.CreatedEvent,
		Payload: proto.Message{
			Role:      proto.User,
			SessionID: "s1",
			Parts: []proto.ContentPart{
				proto.ShellCommand{Command: "ls", Output: "file.go", ExitCode: 0},
			},
		},
	}
	assert.Nil(t, Translate(ev))
}

func TestTranslateProtoUserMessageDeletedIgnored(t *testing.T) {
	t.Parallel()
	ev := pubsub.Event[proto.Message]{
		Type:    pubsub.DeletedEvent,
		Payload: proto.Message{Role: proto.User, SessionID: "s1"},
	}
	assert.Nil(t, Translate(ev))
}

func TestTranslateProtoUserMessageUpdatedIgnored(t *testing.T) {
	t.Parallel()
	// The server preserves the domain event type on the wire
	// (messageToProto), so the creation-only rule has to hold here
	// too.
	ev := pubsub.Event[proto.Message]{
		Type: pubsub.UpdatedEvent,
		Payload: proto.Message{
			Role:      proto.User,
			SessionID: "s1",
			Parts:     []proto.ContentPart{proto.TextContent{Text: "hi"}},
		},
	}
	assert.Nil(t, Translate(ev))
}

func TestTranslateProtoRunComplete(t *testing.T) {
	t.Parallel()
	ev := pubsub.Event[proto.RunComplete]{
		Payload: proto.RunComplete{SessionID: "s1"},
	}
	assert.Equal(t, RunComplete{SessionID: "s1"}, Translate(ev))
}

func TestTranslateProtoPermissionRequest(t *testing.T) {
	t.Parallel()
	ev := pubsub.Event[proto.PermissionRequest]{
		Payload: proto.PermissionRequest{
			ToolName:    "bash",
			ToolCallID:  "tc-1",
			Description: "Execute command: ls",
		},
	}
	assert.Equal(t, PermissionRequested{
		ToolCallID:  "tc-1",
		ToolName:    "bash",
		Description: "Execute command: ls",
	}, Translate(ev))
}

func TestTranslateProtoPermissionNotification(t *testing.T) {
	t.Parallel()
	granted := pubsub.Event[proto.PermissionNotification]{
		Payload: proto.PermissionNotification{ToolCallID: "tc-1", Granted: true},
	}
	assert.Equal(t, PermissionResolved{ToolCallID: "tc-1"}, Translate(granted))

	// The server forwards the permission service's flagless
	// request marker too; it is not a resolution on the wire
	// either.
	marker := pubsub.Event[proto.PermissionNotification]{
		Payload: proto.PermissionNotification{ToolCallID: "tc-1"},
	}
	assert.Nil(t, Translate(marker))
}

func TestTranslateProtoQuestionRequest(t *testing.T) {
	t.Parallel()
	ev := pubsub.Event[proto.QuestionRequest]{
		Payload: proto.QuestionRequest{
			ID:        "b1",
			SessionID: "s1",
			Questions: []proto.QuestionItem{
				{Question: "Pick an option"},
			},
		},
	}
	assert.Equal(t, QuestionAsked{BatchID: "b1", Text: "Pick an option"}, Translate(ev))
}

func TestTranslateProtoQuestionRequestFirstLinesMessage(t *testing.T) {
	t.Parallel()
	// Same single-line contract as the domain path: text past the
	// first newline is dropped and the remainder trimmed.
	ev := pubsub.Event[proto.QuestionRequest]{
		Payload: proto.QuestionRequest{
			ID: "b1",
			Questions: []proto.QuestionItem{
				{Question: " Pick an option\nExplanations follow"},
			},
		},
	}
	assert.Equal(t, QuestionAsked{BatchID: "b1", Text: "Pick an option"}, Translate(ev))
}

func TestTranslateProtoQuestionRequestFirstLineStillTruncated(t *testing.T) {
	t.Parallel()
	ev := pubsub.Event[proto.QuestionRequest]{
		Payload: proto.QuestionRequest{
			Questions: []proto.QuestionItem{
				{Question: strings.Repeat("a", 100) + "\r\nignored"},
			},
		},
	}
	want := QuestionAsked{Text: strings.Repeat("a", maxTextFieldLength)}
	assert.Equal(t, want, Translate(ev))
}

func TestTranslateProtoQuestionNotification(t *testing.T) {
	t.Parallel()
	ev := pubsub.Event[proto.QuestionNotification]{
		Payload: proto.QuestionNotification{BatchID: "b1"},
	}
	assert.Equal(t, QuestionResolved{BatchID: "b1"}, Translate(ev))
}

func TestTranslateProtoReAuthenticateAgentEvent(t *testing.T) {
	t.Parallel()
	// The server wraps notify.Notification into proto.AgentEvent
	// with the domain type string, so re_authenticate arrives as a
	// raw agent event type. ProviderID does not cross the wire.
	ev := pubsub.Event[proto.AgentEvent]{
		Payload: proto.AgentEvent{
			Type: proto.AgentEventType(notify.TypeReAuthenticate),
		},
	}
	assert.Equal(t, AuthRequired{}, Translate(ev))
}

func TestTranslateProtoAgentEventIgnored(t *testing.T) {
	t.Parallel()
	// proto.Message carries no IsSummaryMessage flag and nothing
	// publishes AgentEventTypeSummarize, so summarize agent events
	// never map to a herdr event.
	ev := pubsub.Event[proto.AgentEvent]{
		Payload: proto.AgentEvent{
			Type:    proto.AgentEventTypeSummarize,
			Message: proto.Message{SessionID: "s1"},
		},
	}
	assert.Nil(t, Translate(ev))
}

func TestTranslateDomainSession(t *testing.T) {
	t.Parallel()

	// Created and updated sessions carry the title for the pane
	// presentation; deletions are left to the SetSession clear
	// path when the UI switches away.
	created := pubsub.Event[session.Session]{
		Type:    pubsub.CreatedEvent,
		Payload: session.Session{ID: "s1", Title: "My Title"},
	}
	assert.Equal(t, SessionUpdated{SessionID: "s1", Title: "My Title"}, Translate(created))

	updated := pubsub.Event[session.Session]{
		Type:    pubsub.UpdatedEvent,
		Payload: session.Session{ID: "s1", Title: "Renamed"},
	}
	assert.Equal(t, SessionUpdated{SessionID: "s1", Title: "Renamed"}, Translate(updated))

	deleted := pubsub.Event[session.Session]{
		Type:    pubsub.DeletedEvent,
		Payload: session.Session{ID: "s1"},
	}
	assert.Nil(t, Translate(deleted))
}

func TestTranslateProtoSession(t *testing.T) {
	t.Parallel()

	updated := pubsub.Event[proto.Session]{
		Type:    pubsub.UpdatedEvent,
		Payload: proto.Session{ID: "s1", Title: "My Title"},
	}
	assert.Equal(t, SessionUpdated{SessionID: "s1", Title: "My Title"}, Translate(updated))

	deleted := pubsub.Event[proto.Session]{
		Type:    pubsub.DeletedEvent,
		Payload: proto.Session{ID: "s1"},
	}
	assert.Nil(t, Translate(deleted))
}

// Unknown types.

func TestTranslateUnknownReturnsNil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, Translate("not an event"))
}

// Bridge wiring.

// brokerSubscriber adapts a pubsub.Broker to the plain subscriber
// fields of BridgeSources.
type brokerSubscriber[T any] struct {
	b *pubsub.Broker[T]
}

func (s brokerSubscriber[T]) Subscribe(ctx context.Context) <-chan pubsub.Event[T] {
	return s.b.Subscribe(ctx)
}

// permStub adapts brokers to the permission sources of BridgeSources.
type permStub struct {
	brokerSubscriber[permission.PermissionRequest]
	notifications *pubsub.Broker[permission.PermissionNotification]
}

func (s permStub) SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[permission.PermissionNotification] {
	return s.notifications.Subscribe(ctx)
}

// questionStub adapts brokers to the question sources of
// BridgeSources.
type questionStub struct {
	brokerSubscriber[question.Request]
	notifications *pubsub.Broker[question.Notification]
}

func (s questionStub) SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[question.Notification] {
	return s.notifications.Subscribe(ctx)
}

func TestBridgeLocalForwardsQuestionEvents(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	perms := permStub{
		brokerSubscriber: brokerSubscriber[permission.PermissionRequest]{pubsub.NewBroker[permission.PermissionRequest]()},
		notifications:    pubsub.NewBroker[permission.PermissionNotification](),
	}
	questions := questionStub{
		brokerSubscriber: brokerSubscriber[question.Request]{pubsub.NewBroker[question.Request]()},
		notifications:    pubsub.NewBroker[question.Notification](),
	}
	src := BridgeSources{
		PermRequests:          perms,
		PermNotifications:     perms,
		RunCompletions:        brokerSubscriber[notify.RunComplete]{pubsub.NewBroker[notify.RunComplete]()},
		Messages:              brokerSubscriber[message.Message]{pubsub.NewBroker[message.Message]()},
		Questions:             questions,
		QuestionNotifications: questions,
		Notifications:         brokerSubscriber[notify.Notification]{pubsub.NewBroker[notify.Notification]()},
		Sessions:              brokerSubscriber[session.Session]{pubsub.NewBroker[session.Session]()},
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	BridgeLocal(ctx, c, src)

	// Wait until the bridge has subscribed before publishing so no
	// event is lost.
	assert.Eventually(t, func() bool {
		return questions.b.GetSubscriberCount() > 0 &&
			questions.notifications.GetSubscriberCount() > 0
	}, time.Second, time.Millisecond)

	// A published question request blocks the pane, carrying the
	// first question's text.
	questions.b.Publish(pubsub.CreatedEvent, question.Request{
		ID:        "b1",
		Questions: []question.Question{{Text: "Pick an option"}},
	})
	assert.Eventually(t, func() bool {
		return slices.Equal(reportedStates(c), []string{stateBlocked})
	}, time.Second, time.Millisecond)
	c.mu.Lock()
	assert.Equal(t, "Pick an option", c.message)
	c.mu.Unlock()

	// Its resolution notification, correlated by batch id, unblocks
	// it.
	questions.notifications.Publish(pubsub.CreatedEvent, question.Notification{BatchID: "b1"})
	assert.Eventually(t, func() bool {
		return slices.Equal(reportedStates(c), []string{stateBlocked, stateWorking})
	}, time.Second, time.Millisecond)
}

func TestBridgeLocalForwardsAuthNotification(t *testing.T) {
	t.Parallel()
	c := newTestClient()

	perms := permStub{
		brokerSubscriber: brokerSubscriber[permission.PermissionRequest]{pubsub.NewBroker[permission.PermissionRequest]()},
		notifications:    pubsub.NewBroker[permission.PermissionNotification](),
	}
	questions := questionStub{
		brokerSubscriber: brokerSubscriber[question.Request]{pubsub.NewBroker[question.Request]()},
		notifications:    pubsub.NewBroker[question.Notification](),
	}
	notifications := pubsub.NewBroker[notify.Notification]()
	src := BridgeSources{
		PermRequests:          perms,
		PermNotifications:     perms,
		RunCompletions:        brokerSubscriber[notify.RunComplete]{pubsub.NewBroker[notify.RunComplete]()},
		Messages:              brokerSubscriber[message.Message]{pubsub.NewBroker[message.Message]()},
		Questions:             questions,
		QuestionNotifications: questions,
		Notifications:         brokerSubscriber[notify.Notification]{notifications},
		Sessions:              brokerSubscriber[session.Session]{pubsub.NewBroker[session.Session]()},
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	BridgeLocal(ctx, c, src)

	// Wait until the bridge has subscribed before publishing so no
	// event is lost.
	assert.Eventually(t, func() bool {
		return notifications.GetSubscriberCount() > 0
	}, time.Second, time.Millisecond)

	// A re-authentication notification blocks the pane, naming the
	// provider.
	notifications.Publish(pubsub.CreatedEvent, notify.Notification{
		Type:       notify.TypeReAuthenticate,
		ProviderID: "hyper",
	})
	assert.Eventually(t, func() bool {
		return slices.Equal(reportedStates(c), []string{stateBlocked})
	}, time.Second, time.Millisecond)
	c.mu.Lock()
	assert.Equal(t, "Re-authentication required: hyper", c.message)
	c.mu.Unlock()

	// Other notification types pass through without a report.
	notifications.Publish(pubsub.CreatedEvent, notify.Notification{
		Type: notify.TypeAgentFinished,
	})
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, []string{stateBlocked}, reportedStates(c))
}
