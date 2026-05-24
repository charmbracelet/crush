package workspace

import (
	"testing"

	"github.com/taigrr/crush/internal/message"
	"github.com/taigrr/crush/internal/proto"
	"github.com/taigrr/crush/internal/pubsub"
	"github.com/taigrr/crush/internal/skills"
	"github.com/stretchr/testify/require"
)

// TestProtoToMessageToolResult ensures that ToolResult metadata,
// data, and MIME type survive the conversion from proto on the
// client. Without these fields the TUI cannot render rich tool
// output (e.g. syntax-highlighted code from view, diffs from edit,
// images, etc.) and falls back to the raw LLM-facing string.
func TestProtoToMessageToolResult(t *testing.T) {
	t.Parallel()

	src := proto.Message{
		ID:   "m1",
		Role: proto.Tool,
		Parts: []proto.ContentPart{
			proto.ToolResult{
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

	got := protoToMessage(src)
	require.Len(t, got.Parts, 1)
	tr, ok := got.Parts[0].(message.ToolResult)
	require.True(t, ok, "expected message.ToolResult, got %T", got.Parts[0])
	require.Equal(t, "call-1", tr.ToolCallID)
	require.Equal(t, "view", tr.Name)
	require.Equal(t, "<file>\n  1| hi\n</file>", tr.Content)
	require.Equal(t, "base64data", tr.Data)
	require.Equal(t, "image/png", tr.MIMEType)
	require.Equal(t, `{"file_path":"/tmp/x","content":"hi"}`, tr.Metadata)
	require.False(t, tr.IsError)
}

// TestTranslateEvent_Skills verifies that an incoming proto.SkillEvent
// is converted into pubsub.Event[skills.Event] and that the
// client-process skill cache is updated as a side effect.
func TestTranslateEvent_Skills(t *testing.T) {
	prev := skills.GetLatestStates()
	t.Cleanup(func() { skills.SetLatestStates(prev) })

	skills.SetLatestStates(nil)

	_ = NewClientWorkspace(nil, proto.Workspace{})
	ev := pubsub.Event[proto.SkillEvent]{
		Type: pubsub.UpdatedEvent,
		Payload: proto.SkillEvent{
			States: []proto.SkillState{
				{Name: "from-server", Path: "/p", State: proto.SkillStateNormal},
			},
		},
	}

	out := translateEvent(ev)
	got, ok := out.(pubsub.Event[skills.Event])
	require.True(t, ok, "expected pubsub.Event[skills.Event], got %T", out)
	require.Len(t, got.Payload.States, 1)
	require.Equal(t, "from-server", got.Payload.States[0].Name)
}

// TestNewClientWorkspace does not panic with nil client.
func TestNewClientWorkspace_NilClient(t *testing.T) {
	t.Parallel()
	ws := NewClientWorkspace(nil, proto.Workspace{
		Skills: []proto.SkillState{
			{Name: "seeded", Path: "/p", State: proto.SkillStateNormal},
		},
	})
	require.NotNil(t, ws)
}
