package model

import (
	"testing"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/ui/notification"
	"github.com/stretchr/testify/require"
)

type testNotificationBackend struct {
	notifications []notification.Notification
}

func (b *testNotificationBackend) Send(n notification.Notification) error {
	b.notifications = append(b.notifications, n)
	return nil
}

func TestHandleAgentNotificationUsesFinishedTurnTitle(t *testing.T) {
	ui := testExecutionModeUI(t, `{"options":{"disable_provider_auto_update":true},"tools":{}}`)
	backend := &testNotificationBackend{}
	ui.notifyBackend = backend
	ui.notifyWindowFocused = false
	ui.caps.ReportFocusEvents = true

	cmd := ui.handleAgentNotification(notify.Notification{
		SessionID:    "session-1",
		SessionTitle: "demo",
		Type:         notify.TypeAgentFinished,
	})
	require.NotNil(t, cmd)
	require.Nil(t, cmd())
	require.Len(t, backend.notifications, 1)
	require.Equal(t, "Crush finished turn", backend.notifications[0].Title)
	require.Contains(t, backend.notifications[0].Message, "Agent's turn completed")
}

func TestUpdatePermissionRequestNotificationKeepsWaitingTitle(t *testing.T) {
	ui := testExecutionModeUI(t, `{"options":{"disable_provider_auto_update":true},"tools":{}}`)
	backend := &testNotificationBackend{}
	ui.notifyBackend = backend
	ui.notifyWindowFocused = false
	ui.caps.ReportFocusEvents = true

	_, cmd := ui.Update(pubsub.Event[permission.PermissionRequest]{
		Type: pubsub.CreatedEvent,
		Payload: permission.PermissionRequest{
			ToolName: "view",
		},
	})
	require.NotNil(t, cmd)
	require.Nil(t, cmd())
	require.Len(t, backend.notifications, 1)
	require.Equal(t, "Crush is waiting...", backend.notifications[0].Title)
	require.Contains(t, backend.notifications[0].Message, "Permission required")
}

