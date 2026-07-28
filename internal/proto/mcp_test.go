package proto_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/proto"
	"github.com/stretchr/testify/require"
)

func TestMCPStateString(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		state    proto.MCPState
		expected string
	}{
		"disabled":     {proto.MCPStateDisabled, "disabled"},
		"starting":     {proto.MCPStateStarting, "starting"},
		"connected":    {proto.MCPStateConnected, "connected"},
		"error":        {proto.MCPStateError, "error"},
		"needs auth":   {proto.MCPStateNeedsAuth, "needs auth"},
		"out of range": {proto.MCPState(99), "unknown"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, tc.state.String())
		})
	}
}

func TestMCPStateTextRoundTrip(t *testing.T) {
	t.Parallel()

	states := []proto.MCPState{
		proto.MCPStateDisabled,
		proto.MCPStateStarting,
		proto.MCPStateConnected,
		proto.MCPStateError,
		proto.MCPStateNeedsAuth,
	}

	for _, state := range states {
		t.Run(state.String(), func(t *testing.T) {
			t.Parallel()

			text, err := state.MarshalText()
			require.NoError(t, err)
			require.Equal(t, state.String(), string(text))

			var decoded proto.MCPState
			require.NoError(t, decoded.UnmarshalText(text))
			require.Equal(t, state, decoded)
		})
	}
}

func TestMCPStateUnmarshalTextUnknown(t *testing.T) {
	t.Parallel()

	var state proto.MCPState
	err := state.UnmarshalText([]byte("not a state"))

	require.Error(t, err)
	require.Contains(t, err.Error(), "not a state")
}

// MCPState renders as its text form inside JSON, not as its numeric value.
func TestMCPStateJSON(t *testing.T) {
	t.Parallel()

	data, err := json.Marshal(proto.MCPStateConnected)
	require.NoError(t, err)
	require.JSONEq(t, `"connected"`, string(data))

	var state proto.MCPState
	require.NoError(t, json.Unmarshal([]byte(`"needs auth"`), &state))
	require.Equal(t, proto.MCPStateNeedsAuth, state)
}

func TestMCPEventTypeTextRoundTrip(t *testing.T) {
	t.Parallel()

	types := []proto.MCPEventType{
		proto.MCPEventStateChanged,
		proto.MCPEventToolsListChanged,
		proto.MCPEventPromptsListChanged,
		proto.MCPEventResourcesListChanged,
	}

	for _, eventType := range types {
		t.Run(string(eventType), func(t *testing.T) {
			t.Parallel()

			text, err := eventType.MarshalText()
			require.NoError(t, err)
			require.Equal(t, string(eventType), string(text))

			var decoded proto.MCPEventType
			require.NoError(t, decoded.UnmarshalText(text))
			require.Equal(t, eventType, decoded)
		})
	}
}

func TestMCPEventJSONRoundTrip(t *testing.T) {
	t.Parallel()

	event := proto.MCPEvent{
		Type:          proto.MCPEventStateChanged,
		Name:          "some-server",
		State:         proto.MCPStateConnected,
		ToolCount:     3,
		PromptCount:   2,
		ResourceCount: 1,
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var decoded proto.MCPEvent
	require.NoError(t, json.Unmarshal(data, &decoded))

	require.Equal(t, event.Type, decoded.Type)
	require.Equal(t, event.Name, decoded.Name)
	require.Equal(t, event.State, decoded.State)
	require.Equal(t, event.ToolCount, decoded.ToolCount)
	require.Equal(t, event.PromptCount, decoded.PromptCount)
	require.Equal(t, event.ResourceCount, decoded.ResourceCount)
	require.NoError(t, decoded.Error)
}

// error is not a JSON type, so it crosses the wire as its message and is
// rebuilt on the other side.
func TestMCPEventErrorCrossesTheWire(t *testing.T) {
	t.Parallel()

	event := proto.MCPEvent{
		Type:  proto.MCPEventStateChanged,
		Name:  "some-server",
		State: proto.MCPStateError,
		Error: errors.New("connection refused"),
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)
	require.Contains(t, string(data), "connection refused")

	var decoded proto.MCPEvent
	require.NoError(t, json.Unmarshal(data, &decoded))

	require.Error(t, decoded.Error)
	require.Equal(t, "connection refused", decoded.Error.Error())
}

func TestMCPEventWithoutErrorOmitsIt(t *testing.T) {
	t.Parallel()

	data, err := json.Marshal(proto.MCPEvent{
		Type:  proto.MCPEventStateChanged,
		Name:  "some-server",
		State: proto.MCPStateConnected,
	})
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))
	require.NotContains(t, raw, "error")
}

func TestMCPClientInfoJSONRoundTrip(t *testing.T) {
	t.Parallel()

	connectedAt := time.Date(2024, time.March, 1, 12, 0, 0, 0, time.UTC)
	info := proto.MCPClientInfo{
		Name:          "some-server",
		State:         proto.MCPStateConnected,
		ToolCount:     5,
		PromptCount:   4,
		ResourceCount: 3,
		ConnectedAt:   connectedAt,
	}

	data, err := json.Marshal(info)
	require.NoError(t, err)

	var decoded proto.MCPClientInfo
	require.NoError(t, json.Unmarshal(data, &decoded))

	require.Equal(t, info.Name, decoded.Name)
	require.Equal(t, info.State, decoded.State)
	require.Equal(t, info.ToolCount, decoded.ToolCount)
	require.Equal(t, info.PromptCount, decoded.PromptCount)
	require.Equal(t, info.ResourceCount, decoded.ResourceCount)
	require.True(t, decoded.ConnectedAt.Equal(connectedAt))
	require.NoError(t, decoded.Error)
}

func TestMCPClientInfoErrorCrossesTheWire(t *testing.T) {
	t.Parallel()

	info := proto.MCPClientInfo{
		Name:  "some-server",
		State: proto.MCPStateError,
		Error: errors.New("handshake failed"),
	}

	data, err := json.Marshal(info)
	require.NoError(t, err)

	var decoded proto.MCPClientInfo
	require.NoError(t, json.Unmarshal(data, &decoded))

	require.Error(t, decoded.Error)
	require.Equal(t, "handshake failed", decoded.Error.Error())
}

func TestMCPClientInfoUnmarshalInvalidState(t *testing.T) {
	t.Parallel()

	var info proto.MCPClientInfo
	err := json.Unmarshal([]byte(`{"name":"x","state":"bogus"}`), &info)

	require.Error(t, err)
}
