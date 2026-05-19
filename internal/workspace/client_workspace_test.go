package workspace

import (
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/skills"
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

// TestProtoToSkillStates verifies that the wire representation of skill
// discovery states reconstructs identical values on the client,
// including synthetic errors derived from Error strings.
func TestProtoToSkillStates(t *testing.T) {
	t.Parallel()

	in := []proto.SkillState{
		{Name: "ok", Path: "/p/ok", State: proto.SkillStateNormal},
		{Name: "broken", Path: "/p/broken", State: proto.SkillStateError, Error: "bad frontmatter"},
	}

	got := protoToSkillStates(in)
	require.Len(t, got, 2)
	require.Equal(t, "ok", got[0].Name)
	require.Equal(t, skills.StateNormal, got[0].State)
	require.NoError(t, got[0].Err)
	require.Equal(t, "broken", got[1].Name)
	require.Equal(t, skills.StateError, got[1].State)
	require.EqualError(t, got[1].Err, "bad frontmatter")
}

// TestTranslateEvent_Skills verifies that an incoming proto.SkillsEvent
// is converted into pubsub.Event[skills.Event] and that the
// client-process skill cache is updated as a side effect, so callers
// reading skills.GetLatestStates see fresh data after each delta.
func TestTranslateEvent_Skills(t *testing.T) {
	// Not parallel - touches the package-level skills cache via the
	// manager constructed with WithGlobalMirror.
	prev := skills.GetLatestStates()
	t.Cleanup(func() { skills.SetLatestStates(prev) })

	skills.SetLatestStates(nil)

	w := NewClientWorkspace(nil, proto.Workspace{})
	ev := pubsub.Event[proto.SkillsEvent]{
		Type: pubsub.UpdatedEvent,
		Payload: proto.SkillsEvent{
			States: []proto.SkillState{
				{Name: "from-server", Path: "/p", State: proto.SkillStateNormal},
			},
		},
	}

	out := w.translateEvent(ev)
	got, ok := out.(pubsub.Event[skills.Event])
	require.True(t, ok, "expected pubsub.Event[skills.Event], got %T", out)
	require.Len(t, got.Payload.States, 1)
	require.Equal(t, "from-server", got.Payload.States[0].Name)

	// Manager (with WithGlobalMirror) propagated to the package cache.
	cached := skills.GetLatestStates()
	require.Len(t, cached, 1)
	require.Equal(t, "from-server", cached[0].Name)
}

// TestNewClientWorkspace_SeedsSkillsCache verifies that the snapshot in
// proto.Workspace.Skills populates the package-level cache the TUI
// reads at construction time, eliminating the race between TUI startup
// and the first SSE event.
func TestNewClientWorkspace_SeedsSkillsCache(t *testing.T) {
	// Not parallel - touches the package-level skills cache.
	prev := skills.GetLatestStates()
	t.Cleanup(func() { skills.SetLatestStates(prev) })

	skills.SetLatestStates(nil)

	_ = NewClientWorkspace(nil, proto.Workspace{
		Skills: []proto.SkillState{
			{Name: "seeded", Path: "/p", State: proto.SkillStateNormal},
		},
	})

	got := skills.GetLatestStates()
	require.Len(t, got, 1)
	require.Equal(t, "seeded", got[0].Name)
}
