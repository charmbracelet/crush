package model

// Một tool call ID chỉ duy nhất trong một session ("call_0" lặp lại ở mọi
// session), nên phải khớp theo cả session ID (T10); và tin nhắn của session con
// không có mục cha trong chat đang hiện thì không được bật lại hoạt ảnh (T11).

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/ui/dialog"
)

// Thông báo "đã cấp/từ chối" của s2 chỉ gỡ prompt đang giữ của s2, không gỡ
// prompt của s3 có cùng tool call ID.
func TestResolvedPermissionDropsOnlyItsOwnSessionsHeldPrompt(t *testing.T) {
	t.Parallel()

	m, _ := newPromptUI(t)
	m.heldPerms = map[string]permission.PermissionRequest{
		"s2": {ID: "p2", SessionID: "s2", ToolCallID: "call_0"},
		"s3": {ID: "p3", SessionID: "s3", ToolCallID: "call_0"},
	}
	m.handlePermissionNotification(permission.PermissionNotification{SessionID: "s2", ToolCallID: "call_0", Granted: true})
	require.NotContains(t, m.heldPerms, "s2")
	require.Contains(t, m.heldPerms, "s3", "same tool call ID, other session: still unanswered")

	m.handlePermissionNotification(permission.PermissionNotification{SessionID: "s3", ToolCallID: "call_0", Denied: true})
	require.Empty(t, m.heldPerms)
}

// Hộp thoại xin quyền đang mở cho s1 không bị đóng bởi thông báo của s2 trùng
// tool call ID.
func TestResolvedPermissionOfOtherSessionKeepsOpenDialog(t *testing.T) {
	t.Parallel()

	m, _ := newPromptUI(t)
	m.Update(permissionEventWithToolCall("s1", "p1", "call_0"))
	require.Equal(t, "call_0", openPermToolCall(m))

	m.handlePermissionNotification(permission.PermissionNotification{SessionID: "s2", ToolCallID: "call_0", Granted: true})
	require.True(t, m.dialog.ContainsDialog(dialog.PermissionsID), "another session's call_0 is not this request")

	m.handlePermissionNotification(permission.PermissionNotification{SessionID: "s1", ToolCallID: "call_0", Granted: true})
	require.False(t, m.dialog.ContainsDialog(dialog.PermissionsID))
}

func permissionEventWithToolCall(sessionID, id, toolCallID string) pubsub.Event[permission.PermissionRequest] {
	ev := permEvent(sessionID, id)
	ev.Payload.ToolCallID = toolCallID
	return ev
}

// Tin nhắn của một session con (agent tool) mà chat đang hiện không có mục cha
// của nó thì không làm gì cả, nhất là không bật lại đồng hồ hoạt ảnh vốn đã bị
// tắt khi nạp một session nhàn rỗi.
func TestChildMessageWithoutParentItemKeepsAnimationsOff(t *testing.T) {
	t.Parallel()

	m, _ := newPromptUI(t)
	m.chat.SetAnimationsAllowed(false)
	ev := pubsub.Event[message.Message]{
		Type: pubsub.CreatedEvent,
		Payload: message.Message{
			ID:        "child-msg",
			SessionID: "parent-msg$$call_x", // Cha không có trong chat.
			Role:      message.Assistant,
			Parts:     []message.ContentPart{message.ToolCall{ID: "tc1", Name: "bash", Input: "{}"}},
		},
	}
	require.Nil(t, m.handleChildSessionMessage(ev))
	require.False(t, m.chat.animAllowed, "nothing in this chat is running")
}
