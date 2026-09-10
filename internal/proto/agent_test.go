package proto_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/charmbracelet/crush/internal/proto"
	"github.com/stretchr/testify/require"
)

func TestAgentEventTypeTextRoundTrip(t *testing.T) {
	t.Parallel()

	types := []proto.AgentEventType{
		proto.AgentEventTypeError,
		proto.AgentEventTypeResponse,
		proto.AgentEventTypeSummarize,
	}

	for _, eventType := range types {
		t.Run(string(eventType), func(t *testing.T) {
			t.Parallel()

			text, err := eventType.MarshalText()
			require.NoError(t, err)
			require.Equal(t, string(eventType), string(text))

			var decoded proto.AgentEventType
			require.NoError(t, decoded.UnmarshalText(text))
			require.Equal(t, eventType, decoded)
		})
	}
}

func TestAgentEventJSONRoundTrip(t *testing.T) {
	t.Parallel()

	event := proto.AgentEvent{
		Type:  proto.AgentEventTypeResponse,
		RunID: "run-123",
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var decoded proto.AgentEvent
	require.NoError(t, json.Unmarshal(data, &decoded))

	require.Equal(t, event.Type, decoded.Type)
	require.Equal(t, event.RunID, decoded.RunID)
	require.NoError(t, decoded.Error)
}

// error is not a JSON type, so it crosses the wire as its message and is
// rebuilt on the other side. `crush run` relies on this to attribute a
// failure to the run that caused it.
func TestAgentEventErrorCrossesTheWire(t *testing.T) {
	t.Parallel()

	event := proto.AgentEvent{
		Type:  proto.AgentEventTypeError,
		RunID: "run-123",
		Error: errors.New("model unavailable"),
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)
	require.Contains(t, string(data), "model unavailable")

	var decoded proto.AgentEvent
	require.NoError(t, json.Unmarshal(data, &decoded))

	require.Error(t, decoded.Error)
	require.Equal(t, "model unavailable", decoded.Error.Error())
	require.Equal(t, "run-123", decoded.RunID)
}

func TestAgentEventWithoutErrorOmitsIt(t *testing.T) {
	t.Parallel()

	data, err := json.Marshal(proto.AgentEvent{Type: proto.AgentEventTypeResponse})
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))
	require.NotContains(t, raw, "error")
}

func TestAgentEventSummarizeFields(t *testing.T) {
	t.Parallel()

	event := proto.AgentEvent{
		Type:         proto.AgentEventTypeSummarize,
		SessionID:    "session-1",
		SessionTitle: "a title",
		Progress:     "summarizing",
		Done:         true,
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var decoded proto.AgentEvent
	require.NoError(t, json.Unmarshal(data, &decoded))

	require.Equal(t, event.SessionID, decoded.SessionID)
	require.Equal(t, event.SessionTitle, decoded.SessionTitle)
	require.Equal(t, event.Progress, decoded.Progress)
	require.True(t, decoded.Done)
}

// the optional summarize fields stay off the wire when unset
func TestAgentEventOmitsEmptyOptionalFields(t *testing.T) {
	t.Parallel()

	data, err := json.Marshal(proto.AgentEvent{Type: proto.AgentEventTypeResponse})
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))

	for _, key := range []string{"run_id", "session_id", "session_title", "progress", "done"} {
		require.NotContains(t, raw, key)
	}
}

func TestMessageRoleTextRoundTrip(t *testing.T) {
	t.Parallel()

	roles := []proto.MessageRole{
		proto.Assistant,
		proto.User,
		proto.System,
		proto.Tool,
	}

	for _, role := range roles {
		t.Run(string(role), func(t *testing.T) {
			t.Parallel()

			text, err := role.MarshalText()
			require.NoError(t, err)
			require.Equal(t, string(role), string(text))

			var decoded proto.MessageRole
			require.NoError(t, decoded.UnmarshalText(text))
			require.Equal(t, role, decoded)
		})
	}
}

func TestFinishReasonTextRoundTrip(t *testing.T) {
	t.Parallel()

	reasons := []proto.FinishReason{
		proto.FinishReasonEndTurn,
		proto.FinishReasonMaxTokens,
		proto.FinishReasonToolUse,
		proto.FinishReasonCanceled,
		proto.FinishReasonError,
		proto.FinishReasonUnknown,
	}

	for _, reason := range reasons {
		t.Run(string(reason), func(t *testing.T) {
			t.Parallel()

			text, err := reason.MarshalText()
			require.NoError(t, err)
			require.Equal(t, string(reason), string(text))

			var decoded proto.FinishReason
			require.NoError(t, decoded.UnmarshalText(text))
			require.Equal(t, reason, decoded)
		})
	}
}
