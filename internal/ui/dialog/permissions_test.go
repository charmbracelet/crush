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
	s := styles.CharmtonePantera()
	com := &common.Common{Styles: &s}
	perm := permission.PermissionRequest{
		ID:         "perm-test",
		ToolCallID: "tool-call-test",
		ToolName:   "bash",
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
	require.Equal(t, optionDeny, p.selectedOption, "a fresh dialog starts on Deny")

	// Right cycles forward, wrapping from Deny to Allow.
	p.HandleMsg(tea.KeyPressMsg{Code: tea.KeyRight})
	require.Equal(t, optionAllow, p.selectedOption)

	p.HandleMsg(tea.KeyPressMsg{Code: tea.KeyRight})
	require.Equal(t, optionAllowSession, p.selectedOption)

	p.HandleMsg(tea.KeyPressMsg{Code: tea.KeyRight})
	require.Equal(t, optionDeny, p.selectedOption)

	// Left cycles backward.
	p.HandleMsg(tea.KeyPressMsg{Code: tea.KeyLeft})
	require.Equal(t, optionAllowSession, p.selectedOption)
}

// Hộp thoại có thể bật lên giữa lúc người dùng đang gõ: chữ thường (h, l, s)
// và Tab không được đẩy lựa chọn sang Allow, nên Enter sau đó vẫn là Deny.
func TestPermissions_TypingThenEnterDenies(t *testing.T) {
	t.Parallel()

	typed := func(s string) []tea.KeyPressMsg {
		var keys []tea.KeyPressMsg
		for _, r := range s {
			keys = append(keys, keyMsg(r))
		}
		return keys
	}
	for name, keys := range map[string][]tea.KeyPressMsg{
		"git log":     typed("git log"),
		"fix the bug": typed("fix the bug"),
		"sss":         typed("sss"),
		"tab":         {{Code: tea.KeyTab}, {Code: tea.KeyTab}},
	} {
		p := newTestPermissions(t)
		for _, k := range keys {
			if resp, ok := p.HandleMsg(k).(ActionPermissionResponse); ok {
				require.Equalf(t, PermissionDeny, resp.Action, "%s: a typed key granted permission", name)
			}
		}
		resp, ok := p.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter}).(ActionPermissionResponse)
		require.Truef(t, ok, name)
		require.Equalf(t, PermissionDeny, resp.Action, "%s then enter must deny", name)
	}
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
