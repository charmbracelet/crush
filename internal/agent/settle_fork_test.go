package agent

import (
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

// Session của sub-agent khi session cha đang chạy: tool call chưa có kết quả
// là đang chạy thật, không được ghi "Interrupted". Cha rảnh thì mới settle.
func TestReadSettledMessages_SkipsSubAgentOfBusyParent(t *testing.T) {
	env := testEnv(t)
	ctx := t.Context()

	parent, err := env.sessions.Create(ctx, "parent")
	require.NoError(t, err)
	child, err := env.sessions.CreateTaskSession(ctx, "child-tool-session", parent.ID, "sub-agent")
	require.NoError(t, err)
	_, err = env.messages.Create(ctx, child.ID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{message.ToolCall{
			ID: "call_live", Name: "bash", Input: `{"command":"sleep 30"}`, Finished: true,
		}},
	})
	require.NoError(t, err)

	parentBusy := true
	busy := BusyWithAncestors(ctx, env.sessions, func(id string) bool { return parentBusy && id == parent.ID })

	msgs, err := ReadSettledMessages(ctx, env.messages, busy, child.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 1, "a live sub-agent call must not be settled")

	parentBusy = false
	msgs, err = ReadSettledMessages(ctx, env.messages, busy, child.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 2, "once the parent is idle the orphan is settled")
	require.Equal(t, interruptedToolResult, msgs[1].ToolResults()[0].Content)
}

// Không đọc được session (vd đã xoá) thì coi là bận: không settle.
func TestBusyWithAncestors_UnknownSessionIsBusy(t *testing.T) {
	env := testEnv(t)
	busy := BusyWithAncestors(t.Context(), env.sessions, func(string) bool { return false })
	require.True(t, busy("missing"))
	require.Nil(t, BusyWithAncestors(t.Context(), env.sessions, nil), "no agent: nothing can be running")
}
