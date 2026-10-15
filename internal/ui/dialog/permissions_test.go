package dialog

import (
	"image"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
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

// ctrlKey là phím Ctrl+<r> như terminal gửi lên.
func ctrlKey(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Mod: tea.ModCtrl}
}

// TestPermissions_ActionKeysResolve verifies that action keys produce the
// correct permission response. Cho phép chỉ qua Ctrl+A / Ctrl+S; từ chối qua
// d/D và Esc.
func TestPermissions_ActionKeysResolve(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		key    tea.KeyPressMsg
		action PermissionAction
	}{
		{"ctrl+a", ctrlKey('a'), PermissionAllow},
		{"ctrl+s", ctrlKey('s'), PermissionAllowForSession},
		{"d", keyMsg('d'), PermissionDeny},
		{"D", keyMsg('D'), PermissionDeny},
		{"esc", tea.KeyPressMsg{Code: tea.KeyEscape}, PermissionDeny},
	}

	for _, tc := range tests {
		p := newTestPermissions(t)
		action := p.HandleMsg(tc.key)
		resp, ok := action.(ActionPermissionResponse)
		require.Truef(t, ok, "key %s should produce ActionPermissionResponse", tc.name)
		require.Equalf(t, tc.action, resp.Action, "key %s", tc.name)
	}
}

// Chữ trần (a, A, s, S, y, Y...) và phím Alt không được cấp quyền: hộp thoại
// có thể bật lên giữa lúc người dùng đang gõ, nên mọi phím in được phải vô
// hại. Chỉ Ctrl+A / Ctrl+S (hoặc di chuyển rồi Enter) mới cho phép.
func TestPermissions_PlainLettersNeverGrant(t *testing.T) {
	t.Parallel()

	for _, r := range "aAsSyYnNhHlL " {
		p := newTestPermissions(t)
		require.Nilf(t, p.HandleMsg(keyMsg(r)), "plain %q must do nothing", r)
		require.Equalf(t, optionDeny, p.selectedOption, "plain %q must not move the highlight", r)
	}
	for _, k := range []tea.KeyPressMsg{
		{Code: 'a', Mod: tea.ModAlt},
		{Code: 's', Mod: tea.ModAlt},
		{Code: 'a', Mod: tea.ModCtrl | tea.ModShift},
	} {
		p := newTestPermissions(t)
		require.Nilf(t, p.HandleMsg(k), "%s must do nothing", k.String())
	}
}

// TestPermissions_ShortHelpAdvertisesCtrlKeys: phím cho phép phải hiện trên
// dòng gợi ý, vì không còn chữ gạch chân nào trên nút để người dùng đoán.
func TestPermissions_ShortHelpAdvertisesCtrlKeys(t *testing.T) {
	t.Parallel()

	got := map[string]string{}
	for _, b := range newTestPermissions(t).ShortHelp() {
		got[b.Help().Key] = b.Help().Desc
	}
	require.Equal(t, "allow once", got["ctrl+a"])
	require.Equal(t, "allow session", got["ctrl+s"])
	require.NotContains(t, got, "a", "the old bare-letter hint is gone")
	require.NotContains(t, got, "S")
}

// Ở terminal 80 cột hộp thoại chỉ rộng khoảng 48 ô và dòng gợi ý bị cắt từ cuối:
// hai phím cho phép phải còn nằm trong phần nhìn thấy, vì không còn chữ gạch
// chân nào trên nút để người dùng đoán.
func TestPermissions_DrawShowsAllowHintsAt80Columns(t *testing.T) {
	t.Parallel()

	p := newTestPermissions(t)
	scr := uv.NewScreenBuffer(80, 24)
	p.Draw(scr, image.Rect(0, 0, 80, 24))
	text := screenText(scr)
	require.Contains(t, text, "ctrl+a allow once")
	require.Contains(t, text, "ctrl+s allow session")
	require.Contains(t, text, "Allow")
	require.Contains(t, text, "Deny")
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

// Hộp thoại có thể bật lên giữa lúc người dùng đang gõ (nó chỉ nuốt phím trong
// ≤1,5 s): cả câu đầy chữ a/A/s/S/y không được cấp quyền, và Enter sau đó vẫn
// là Deny. Chỉ chữ d/D (từ chối) được phép trả lời.
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
		"git log":        typed("git log"),
		"fix the bug":    typed("fix the bug"),
		"sss":            typed("sss"),
		"and add tests":  typed("and add tests"),
		"yes save all":   typed("yes, Save it for the Session and Allow all"),
		"shouting":       typed("AAA SSS YYY aaa sss yyy"),
		"tab":            {{Code: tea.KeyTab}, {Code: tea.KeyTab}},
		"h and l":        typed("hhhlll"),
		"y then enter":   typed("y"),
		"capital S only": typed("S"),
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
