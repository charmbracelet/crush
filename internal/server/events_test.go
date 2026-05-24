package server

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/taigrr/crush/internal/message"
	"github.com/taigrr/crush/internal/proto"
	"github.com/taigrr/crush/internal/pubsub"
	"github.com/taigrr/crush/internal/skills"
	"github.com/stretchr/testify/require"
)

// TestMessageToProtoToolResult ensures that ToolResult metadata,
// data, and MIME type survive the conversion to proto. Without these
// fields the TUI cannot render rich tool output (e.g. syntax-
// highlighted code from view, diffs from edit, images, etc.) and
// falls back to the raw LLM-facing string.
func TestMessageToProtoToolResult(t *testing.T) {
	t.Parallel()

	src := message.Message{
		ID:   "m1",
		Role: message.Tool,
		Parts: []message.ContentPart{
			message.ToolResult{
				ToolCallID: "call-1",
				Name:       "view",
				Content:    "<file>\n  1| hi\n</file>",
				Data:       "base64data",
				MIMEType:   "image/png",
				Metadata:   `{"file_path":"/tmp/x","content":"hi"}`,
				IsError:    false,
			},
		},
	}

	got := messageToProto(src)
	require.Len(t, got.Parts, 1)
	tr, ok := got.Parts[0].(proto.ToolResult)
	require.True(t, ok, "expected proto.ToolResult, got %T", got.Parts[0])
	require.Equal(t, "call-1", tr.ToolCallID)
	require.Equal(t, "view", tr.Name)
	require.Equal(t, "<file>\n  1| hi\n</file>", tr.Content)
	require.Equal(t, "base64data", tr.Data)
	require.Equal(t, "image/png", tr.MIMEType)
	require.Equal(t, `{"file_path":"/tmp/x","content":"hi"}`, tr.Metadata)
	require.False(t, tr.IsError)
}

// TestSkillsEventToProto_RoundTrip verifies that a pubsub.Event[skills.Event]
// can be wrapped, marshaled, and unmarshaled back through the SSE
// envelope without losing state values or error messages.
func TestSkillsEventToProto_RoundTrip(t *testing.T) {
	t.Parallel()

	src := pubsub.Event[skills.Event]{
		Type: pubsub.UpdatedEvent,
		Payload: skills.Event{
			States: []*skills.SkillState{
				{Name: "ok", Path: "/p/ok", State: skills.StateNormal},
				{Name: "broken", Path: "/p/broken", State: skills.StateError, Err: errors.New("bad frontmatter")},
			},
		},
	}

	env := wrapEvent(src)
	require.NotNil(t, env)
	require.Equal(t, pubsub.PayloadTypeSkillsEvent, env.Type)

	var decoded pubsub.Event[proto.SkillsEvent]
	require.NoError(t, json.Unmarshal(env.Payload, &decoded))
	require.Equal(t, pubsub.UpdatedEvent, decoded.Type)
	require.Len(t, decoded.Payload.States, 2)

	require.Equal(t, "ok", decoded.Payload.States[0].Name)
	require.Equal(t, "/p/ok", decoded.Payload.States[0].Path)
	require.Equal(t, proto.SkillStateNormal, decoded.Payload.States[0].State)
	require.Empty(t, decoded.Payload.States[0].Error)

	require.Equal(t, "broken", decoded.Payload.States[1].Name)
	require.Equal(t, proto.SkillStateError, decoded.Payload.States[1].State)
	require.EqualError(t, decoded.Payload.States[1].Error, "bad frontmatter")
}
