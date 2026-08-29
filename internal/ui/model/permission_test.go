package model

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/stretchr/testify/require"
)

// newTestUIForPermissions builds a UI with a chat, dialog overlay, and
// common context sufficient to exercise the permission dialog paths.
// openPermissionsDialog reads the diff mode out of the config, so the
// workspace has to answer Config().
func newTestUIForPermissions() *UI {
	u := newTestUI()
	u.com.Workspace = &testWorkspace{cfg: &config.Config{
		Options: &config.Options{TUI: &config.TUIOptions{}},
	}}
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

// permissionDialogToolCallID reports the tool call the open permissions
// dialog is asking about, or "" when no dialog is open.
func permissionDialogToolCallID(u *UI) string {
	d := u.dialog.Dialog(dialog.PermissionsID)
	if d == nil {
		return ""
	}
	perm, ok := d.(*dialog.Permissions)
	if !ok {
		return ""
	}
	return perm.ToolCallID()
}

// TestOpenPermissionsDialog_QueuesInsteadOfReplacing pins the counterpart
// of dropping the workspace-wide lock in the permission service.
// Requests are no longer serialized, so two can be waiting at once —
// parallel tools run concurrently. The dialog used to replace whatever
// was on screen, which left the replaced request with nothing that could
// answer it: its tool call blocked until the run was cancelled.
func TestOpenPermissionsDialog_QueuesInsteadOfReplacing(t *testing.T) {
	t.Parallel()

	u := newTestUIForPermissions()
	first := permission.PermissionRequest{ID: "perm-1", ToolCallID: "call-1", ToolName: "bash"}
	second := permission.PermissionRequest{ID: "perm-2", ToolCallID: "call-2", ToolName: "download"}

	u.openPermissionsDialog(first)
	u.openPermissionsDialog(second)

	require.Equal(t, "call-1", permissionDialogToolCallID(u),
		"the request already on screen must keep the dialog")
	require.Len(t, u.pendingPermissions, 1)

	// Answering the first one hands the dialog to the queued request.
	u.closePermissionsDialog()
	require.Equal(t, "call-2", permissionDialogToolCallID(u),
		"the queued request must get the dialog once the first is answered")
	require.Empty(t, u.pendingPermissions)

	u.closePermissionsDialog()
	require.Empty(t, permissionDialogToolCallID(u))
}

// TestHandlePermissionNotification_DropsResolvedQueuedRequest covers a
// queued request that is answered elsewhere — by another client, or by
// its run being cancelled. Its dialog must never open: it would ask
// about a tool call that has already moved on.
func TestHandlePermissionNotification_DropsResolvedQueuedRequest(t *testing.T) {
	t.Parallel()

	u := newTestUIForPermissions()
	u.openPermissionsDialog(permission.PermissionRequest{ID: "perm-1", ToolCallID: "call-1"})
	u.openPermissionsDialog(permission.PermissionRequest{ID: "perm-2", ToolCallID: "call-2"})
	u.openPermissionsDialog(permission.PermissionRequest{ID: "perm-3", ToolCallID: "call-3"})
	require.Len(t, u.pendingPermissions, 2)

	u.handlePermissionNotification(permission.PermissionNotification{
		ToolCallID: "call-2",
		Granted:    true,
	})
	require.Equal(t, "call-1", permissionDialogToolCallID(u),
		"a resolution for a queued request must not disturb the open dialog")
	require.Len(t, u.pendingPermissions, 1)

	u.closePermissionsDialog()
	require.Equal(t, "call-3", permissionDialogToolCallID(u),
		"the dropped request must be skipped")
}

// TestHandlePermissionNotification_OpensTheQueuedRequest checks the
// remote-resolution path also advances the queue, so a request answered
// from another client does not leave the queued ones stranded.
func TestHandlePermissionNotification_OpensTheQueuedRequest(t *testing.T) {
	t.Parallel()

	u := newTestUIForPermissions()
	u.openPermissionsDialog(permission.PermissionRequest{ID: "perm-1", ToolCallID: "call-1"})
	u.openPermissionsDialog(permission.PermissionRequest{ID: "perm-2", ToolCallID: "call-2"})

	u.handlePermissionNotification(permission.PermissionNotification{
		ToolCallID: "call-1",
		Denied:     true,
	})

	require.Equal(t, "call-2", permissionDialogToolCallID(u))
	require.Empty(t, u.pendingPermissions)
}

// TestHandlePermissionNotification_EmptyToolCallIDIsIgnored pins the
// guard against a notification that names no tool call. Matching on an
// empty ID dismissed whichever dialog or queued request also had no ID,
// which is how a live two-download turn lost its second dialog before
// the download tool started sending its tool call ID.
func TestHandlePermissionNotification_EmptyToolCallIDIsIgnored(t *testing.T) {
	t.Parallel()

	u := newTestUIForPermissions()
	u.openPermissionsDialog(permission.PermissionRequest{ID: "perm-1"})
	u.openPermissionsDialog(permission.PermissionRequest{ID: "perm-2"})

	u.handlePermissionNotification(permission.PermissionNotification{Granted: true})

	require.NotNil(t, u.dialog.Dialog(dialog.PermissionsID),
		"an unidentified resolution must not close the dialog")
	require.Len(t, u.pendingPermissions, 1,
		"an unidentified resolution must not drop a queued request")
}
