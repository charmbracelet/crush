package model

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/stretchr/testify/require"
)

// newTestUIForPermissions builds a UI with a chat, dialog overlay, and
// common context sufficient to exercise handlePermissionNotification.
func newTestUIForPermissions() *UI {
	u := newTestUI()
	u.dialog = dialog.NewOverlay()
	return u
}

func TestHandlePermissionNotification_RemoteGrantClosesDialog(t *testing.T) {
	t.Parallel()

	u := newTestUIForPermissions()
	perm := permission.PermissionRequest{
		ID:         "perm-1",
		ToolCallID: "tool-call-X",
		ToolName:   "bash",
	}
	u.dialog.OpenDialogWithGrace(dialog.NewPermissions(u.com, perm))
	require.True(t, u.dialog.ContainsDialog(dialog.PermissionsID))

	u.handlePermissionNotification(permission.PermissionNotification{
		ToolCallID: "tool-call-X",
		Granted:    true,
	})

	require.False(t, u.dialog.ContainsDialog(dialog.PermissionsID),
		"granted notification should close matching permissions dialog")
}

func TestHandlePermissionNotification_RemoteDenyClosesDialog(t *testing.T) {
	t.Parallel()

	u := newTestUIForPermissions()
	perm := permission.PermissionRequest{
		ID:         "perm-2",
		ToolCallID: "tool-call-Y",
	}
	u.dialog.OpenDialogWithGrace(dialog.NewPermissions(u.com, perm))

	u.handlePermissionNotification(permission.PermissionNotification{
		ToolCallID: "tool-call-Y",
		Denied:     true,
	})

	require.False(t, u.dialog.ContainsDialog(dialog.PermissionsID),
		"denied notification should close matching permissions dialog")
}

func TestHandlePermissionNotification_InitialPendingDoesNotClose(t *testing.T) {
	t.Parallel()

	u := newTestUIForPermissions()
	perm := permission.PermissionRequest{
		ID:         "perm-3",
		ToolCallID: "tool-call-Z",
	}
	u.dialog.OpenDialogWithGrace(dialog.NewPermissions(u.com, perm))

	// The initial notification published by permission.Request is
	// neither granted nor denied; it must not dismiss the dialog.
	u.handlePermissionNotification(permission.PermissionNotification{
		ToolCallID: "tool-call-Z",
	})

	require.True(t, u.dialog.ContainsDialog(dialog.PermissionsID),
		"initial pending notification must not close the dialog")
}

func TestHandlePermissionNotification_DifferentToolCallIDDoesNotClose(t *testing.T) {
	t.Parallel()

	u := newTestUIForPermissions()
	perm := permission.PermissionRequest{
		ID:         "perm-4",
		ToolCallID: "tool-call-A",
	}
	u.dialog.OpenDialogWithGrace(dialog.NewPermissions(u.com, perm))

	u.handlePermissionNotification(permission.PermissionNotification{
		ToolCallID: "tool-call-B",
		Granted:    true,
	})

	require.True(t, u.dialog.ContainsDialog(dialog.PermissionsID),
		"notification for unrelated tool call must not close the dialog")
}

// permissionWorkspace is a countingWorkspace with the config the
// permissions dialog reads its diff mode from.
type permissionWorkspace struct {
	*countingWorkspace
}

func (permissionWorkspace) Config() *config.Config {
	return &config.Config{Options: &config.Options{TUI: &config.TUIOptions{}}}
}

// permissionEvent is the event the workspace publishes when a tool asks for
// the permission with the given ID, and again whenever the request is
// replayed.
func permissionEvent(id string) pubsub.Event[permission.PermissionRequest] {
	return pubsub.Event[permission.PermissionRequest]{
		Type: pubsub.CreatedEvent,
		Payload: permission.PermissionRequest{
			ID:         id,
			SessionID:  "s1",
			ToolCallID: "tool-call-" + id,
			ToolName:   "bash",
		},
	}
}

func TestPermissionRequestReplayKeepsDialog(t *testing.T) {
	t.Setenv("CRUSH_PERMISSIONS_PIPED", "")
	u := newProgramStatusUI(t)
	u.com.Workspace = permissionWorkspace{u.com.Workspace.(*countingWorkspace)}
	notifier := &recordingNotifier{}
	u.notifyBackend = notifier
	u.caps.ReportFocusEvents = true
	u.notifyWindowFocused = false

	_, _ = u.Update(permissionEvent("perm-1"))
	dlg := u.dialog.Dialog(dialog.PermissionsID)
	require.NotNil(t, dlg)
	require.Len(t, notifier.sent, 1)

	// The server re-sends a pending request whenever a client reports its
	// session; the replay must keep the open dialog, with its scroll, diff
	// mode, and selected button, and not notify again.
	_, _ = u.Update(permissionEvent("perm-1"))
	require.Same(t, dlg, u.dialog.Dialog(dialog.PermissionsID))
	require.Len(t, notifier.sent, 1)

	_, _ = u.Update(permissionEvent("perm-2"))
	require.NotSame(t, dlg, u.dialog.Dialog(dialog.PermissionsID))
	require.Len(t, notifier.sent, 2)
}
