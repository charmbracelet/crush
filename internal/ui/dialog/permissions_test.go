package dialog

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func newTestPermissions(t *testing.T) *Permissions {
	t.Helper()
	s := styles.CharmtonePantera()
	com := &common.Common{Styles: &s}
	perm := permission.PermissionRequest{
		ID:         "perm-test",
		ToolCallID: "tool-call-test",
		ToolName:   "bash",
	}
	return NewPermissions(com, perm)
}

func newTestDiffPermissions(t *testing.T) *Permissions {
	t.Helper()
	s := styles.CharmtonePantera()
	com := &common.Common{Styles: &s}
	perm := permission.PermissionRequest{
		ID:         "perm-diff-test",
		ToolCallID: "tool-call-diff-test",
		ToolName:   "edit",
		Params: tools.EditPermissionsParams{
			FilePath:   "main.go",
			OldContent: "hello",
			NewContent: "world",
		},
	}
	return NewPermissions(com, perm)
}

// TestPermissions_ActionKeysResolve verifies that action keys produce the
// correct permission response.
func TestPermissions_ActionKeysResolve(t *testing.T) {
	t.Parallel()

	tests := []struct {
		key    tea.KeyPressMsg
		action PermissionAction
	}{
		{keyMsg('a'), PermissionAllow},
		{keyMsg('A'), PermissionAllow},
		{keyMsg('d'), PermissionDeny},
		{keyMsg('D'), PermissionDeny},
		{keyMsg('s'), PermissionAllowForSession},
		{keyMsg('S'), PermissionAllowForSession},
	}

	for _, tc := range tests {
		p := newTestPermissions(t)
		action := p.HandleMsg(tc.key)
		resp, ok := action.(ActionPermissionResponse)
		require.Truef(t, ok, "key %q should produce ActionPermissionResponse", tc.key.Text)
		require.Equal(t, tc.action, resp.Action)
	}
}

// TestPermissions_NavigationCyclesOptions verifies that tab and arrow keys
// cycle through the three permission options.
func TestPermissions_NavigationCyclesOptions(t *testing.T) {
	t.Parallel()

	p := newTestPermissions(t)
	require.Equal(t, 0, p.selectedOption)

	// Tab cycles forward.
	p.HandleMsg(tea.KeyPressMsg{Code: tea.KeyTab})
	require.Equal(t, 1, p.selectedOption)

	p.HandleMsg(tea.KeyPressMsg{Code: tea.KeyTab})
	require.Equal(t, 2, p.selectedOption)

	// Wrap around.
	p.HandleMsg(tea.KeyPressMsg{Code: tea.KeyTab})
	require.Equal(t, 0, p.selectedOption)

	// Left cycles backward.
	p.HandleMsg(keyMsg('h'))
	require.Equal(t, 2, p.selectedOption)
}

// TestPermissions_EnterConfirmsSelection verifies that enter confirms the
// currently selected option.
func TestPermissions_EnterConfirmsSelection(t *testing.T) {
	t.Parallel()

	p := newTestPermissions(t)
	p.selectedOption = 1 // Allow for session.

	action := p.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	resp, ok := action.(ActionPermissionResponse)
	require.True(t, ok)
	require.Equal(t, PermissionAllowForSession, resp.Action)
}

// TestPermissions_EscapeDenies verifies that escape denies the request.
func TestPermissions_EscapeDenies(t *testing.T) {
	t.Parallel()

	p := newTestPermissions(t)
	action := p.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEscape})
	resp, ok := action.(ActionPermissionResponse)
	require.True(t, ok)
	require.Equal(t, PermissionDeny, resp.Action)
}

// TestPermissions_MinimizeTogglesDiffDialog verifies that pressing m on a diff
// permission dialog toggles the minimized state.
func TestPermissions_MinimizeTogglesDiffDialog(t *testing.T) {
	t.Parallel()

	p := newTestDiffPermissions(t)
	require.False(t, p.IsMinimized())

	p.HandleMsg(keyMsg('m'))
	require.True(t, p.IsMinimized())

	p.HandleMsg(keyMsg('m'))
	require.False(t, p.IsMinimized())
}

// TestPermissions_MinimizeBlocksActionsWhenMinimized verifies that action keys
// are ignored while the dialog is minimized.
func TestPermissions_MinimizeBlocksActionsWhenMinimized(t *testing.T) {
	t.Parallel()

	actionKeys := []tea.KeyPressMsg{
		keyMsg('a'), keyMsg('d'), keyMsg('s'),
		{Code: tea.KeyEnter},
		{Code: tea.KeyEscape},
	}

	for _, k := range actionKeys {
		p := newTestDiffPermissions(t)
		p.HandleMsg(keyMsg('m'))
		require.True(t, p.IsMinimized())

		action := p.HandleMsg(k)
		require.Nil(t, action, "key %q should be blocked when minimized", k.Text)
	}
}

// TestPermissions_MinimizeOnlyWorksWithDiffView verifies that m does nothing
// on a non-diff permission dialog (e.g. bash).
func TestPermissions_MinimizeOnlyWorksWithDiffView(t *testing.T) {
	t.Parallel()

	p := newTestPermissions(t) // bash — no diff view
	p.HandleMsg(keyMsg('m'))
	require.False(t, p.IsMinimized())
}

// TestPermissions_HasVisibleDialogsRespectsMinimized verifies that the overlay
// considers a minimized diff dialog as not visible.
func TestPermissions_HasVisibleDialogsRespectsMinimized(t *testing.T) {
	t.Parallel()

	overlay := NewOverlay()
	overlay.OpenDialog(newTestDiffPermissions(t))

	require.True(t, overlay.HasVisibleDialogs())

	perm := overlay.Dialog(PermissionsID).(*Permissions)
	perm.HandleMsg(keyMsg('m'))

	require.True(t, overlay.HasDialogs())
	require.False(t, overlay.HasVisibleDialogs())

	perm.HandleMsg(keyMsg('m'))
	require.True(t, overlay.HasVisibleDialogs())
}
