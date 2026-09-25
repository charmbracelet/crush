package dialog

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func newTestPermissions(t *testing.T) *Permissions {
	t.Helper()
	return newTestPermissionsWithDanger(t, "")
}

// newTestPermissionsWithDanger builds a dialog for a request flagged with
// the given danger reason. An empty reason means an ordinary request.
func newTestPermissionsWithDanger(t *testing.T, danger string) *Permissions {
	t.Helper()
	s := styles.CharmtonePantera()
	com := &common.Common{Styles: &s}
	perm := permission.PermissionRequest{
		ID:         "perm-test",
		ToolCallID: "tool-call-test",
		ToolName:   "bash",
		Danger:     danger,
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

// TestPermissions_DefaultSelectionAvoidsApprovingDangerousRequests verifies
// that an ordinary request starts on Allow while a dangerous one starts on
// Deny, so a reflexive enter refuses instead of approving.
func TestPermissions_DefaultSelectionAvoidsApprovingDangerousRequests(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		danger string
		action PermissionAction
	}{
		{"ordinary request", "", PermissionAllow},
		{"dangerous request", "sudo", PermissionDeny},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			p := newTestPermissionsWithDanger(t, tc.danger)
			require.Equal(t, optionIndex(tc.action), p.selectedOption)

			// The selection is only meaningful if confirming it lands on
			// the matching action.
			action := p.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
			resp, ok := action.(ActionPermissionResponse)
			require.True(t, ok)
			require.Equal(t, tc.action, resp.Action)
		})
	}
}

// TestPermissions_CtrlYDoesNotRespond verifies that ctrl+y, the global
// yolo-mode toggle, does not resolve the dialog. Dialogs see keys before
// the global handler, so accepting it here would silently approve whatever
// is on screen.
func TestPermissions_CtrlYDoesNotRespond(t *testing.T) {
	t.Parallel()

	for _, danger := range []string{"", "sudo"} {
		p := newTestPermissionsWithDanger(t, danger)
		action := p.HandleMsg(tea.KeyPressMsg{Code: 'y', Mod: tea.ModCtrl})
		require.Nil(t, action, "ctrl+y should not resolve the permission dialog")
	}
}

// TestPermissions_DangerousButtonsLookDifferent verifies that the approving
// buttons are styled distinctly on a dangerous request, so the prompt does
// not look identical to a routine one.
func TestPermissions_DangerousButtonsLookDifferent(t *testing.T) {
	t.Parallel()

	ordinary := newTestPermissionsWithDanger(t, "")
	dangerous := newTestPermissionsWithDanger(t, "sudo")

	// Compare the same selection so only the danger styling differs.
	ordinary.selectedOption = optionIndex(PermissionAllow)
	dangerous.selectedOption = optionIndex(PermissionAllow)

	require.NotEqual(t, ordinary.renderButtonGroup("  "), dangerous.renderButtonGroup("  "))
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
