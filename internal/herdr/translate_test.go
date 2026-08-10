package herdr

import (
	"testing"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
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

func TestTranslateProtoAgentEventIgnored(t *testing.T) {
	t.Parallel()
	// proto.Message carries no IsSummaryMessage flag and nothing
	// publishes AgentEventTypeSummarize, so proto agent events never
	// map to a herdr event.
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
