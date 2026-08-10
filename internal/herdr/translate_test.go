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
	"github.com/stretchr/testify/assert"
)

// Domain type translation.

func TestTranslateDomainAssistantMessage(t *testing.T) {
	t.Parallel()
	ev := pubsub.Event[message.Message]{
		Payload: message.Message{Role: message.Assistant, SessionID: "s1"},
	}
	assert.Equal(t, AssistantMessage{SessionID: "s1"}, Translate(ev))
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
		Payload: permission.PermissionRequest{ToolName: "bash"},
	}
	assert.Equal(t, PermissionRequested{}, Translate(ev))
}

func TestTranslateDomainPermissionNotification(t *testing.T) {
	t.Parallel()
	ev := pubsub.Event[permission.PermissionNotification]{
		Payload: permission.PermissionNotification{Granted: true},
	}
	assert.Equal(t, PermissionResolved{}, Translate(ev))
}

func TestTranslateDomainQuestionRequest(t *testing.T) {
	t.Parallel()
	ev := pubsub.Event[question.Request]{
		Payload: question.Request{
			SessionID: "s1",
			Questions: []question.Question{
				{Text: "Which file should I edit?"},
				{Text: "And why?"},
			},
		},
	}
	// The blocked message is the first question's text.
	assert.Equal(t, QuestionAsked{Text: "Which file should I edit?"}, Translate(ev))
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
	want := QuestionAsked{Text: strings.Repeat("界", maxBlockMessageLength)}
	assert.Equal(t, want, got)
}

func TestTranslateDomainQuestionNotification(t *testing.T) {
	t.Parallel()
	ev := pubsub.Event[question.Notification]{
		Payload: question.Notification{BatchID: "b1"},
	}
	assert.Equal(t, QuestionResolved{}, Translate(ev))
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
		Payload: proto.Message{Role: proto.Assistant, SessionID: "s1"},
	}
	assert.Equal(t, AssistantMessage{SessionID: "s1"}, Translate(ev))
}

func TestTranslateProtoNonAssistantIgnored(t *testing.T) {
	t.Parallel()
	ev := pubsub.Event[proto.Message]{
		Payload: proto.Message{Role: proto.User, SessionID: "s1"},
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
		Payload: proto.PermissionRequest{ToolName: "bash"},
	}
	assert.Equal(t, PermissionRequested{}, Translate(ev))
}

func TestTranslateProtoPermissionNotification(t *testing.T) {
	t.Parallel()
	ev := pubsub.Event[proto.PermissionNotification]{
		Payload: proto.PermissionNotification{Granted: true},
	}
	assert.Equal(t, PermissionResolved{}, Translate(ev))
}

func TestTranslateProtoQuestionRequest(t *testing.T) {
	t.Parallel()
	ev := pubsub.Event[proto.QuestionRequest]{
		Payload: proto.QuestionRequest{
			SessionID: "s1",
			Questions: []proto.QuestionItem{
				{Question: "Pick an option"},
			},
		},
	}
	assert.Equal(t, QuestionAsked{Text: "Pick an option"}, Translate(ev))
}

func TestTranslateProtoQuestionNotification(t *testing.T) {
	t.Parallel()
	ev := pubsub.Event[proto.QuestionNotification]{
		Payload: proto.QuestionNotification{BatchID: "b1"},
	}
	assert.Equal(t, QuestionResolved{}, Translate(ev))
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

func TestTranslateProtoSessionIgnored(t *testing.T) {
	t.Parallel()
	ev := pubsub.Event[proto.Session]{
		Payload: proto.Session{ID: "s1"},
	}
	assert.Nil(t, Translate(ev))
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
		Questions: []question.Question{{Text: "Pick an option"}},
	})
	assert.Eventually(t, func() bool {
		return slices.Equal(reportedStates(c), []string{stateBlocked})
	}, time.Second, time.Millisecond)
	c.mu.Lock()
	assert.Equal(t, "Pick an option", c.message)
	c.mu.Unlock()

	// Its resolution notification unblocks it.
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
