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

func TestTranslateDomainSummaryMessage(t *testing.T) {
	t.Parallel()
	ev := pubsub.Event[message.Message]{
		Payload: message.Message{
			Role:             message.Assistant,
			SessionID:        "s1",
			IsSummaryMessage: true,
		},
	}
	assert.Equal(t, Summarizing{}, Translate(ev))
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

func TestTranslateProtoSummarizing(t *testing.T) {
	t.Parallel()
	ev := pubsub.Event[proto.AgentEvent]{
		Payload: proto.AgentEvent{
			Type: proto.AgentEventTypeSummarize,
			Done: false,
		},
	}
	assert.Equal(t, Summarizing{}, Translate(ev))
}

func TestTranslateProtoSummarizeDoneIgnored(t *testing.T) {
	t.Parallel()
	ev := pubsub.Event[proto.AgentEvent]{
		Payload: proto.AgentEvent{
			Type: proto.AgentEventTypeSummarize,
			Done: true,
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
