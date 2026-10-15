package model

// Alt+W và mục "Home" của bảng lệnh (T8): ở chế độ local, đóng workspace là kết
// thúc run, nên lúc agent đang bận phải hỏi xác nhận như Ctrl+C; ở chế độ
// client/server server giữ run nên thoát ngay.

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/ui/dialog"
)

// serverModeWS và localModeWS là workspace của hai chế độ, phân biệt qua
// RoutesChannelEvents (true chỉ ở chế độ client/server).
type (
	serverModeWS struct{ *countingWorkspace }
	localModeWS  struct{ *countingWorkspace }
)

func (serverModeWS) RoutesChannelEvents() bool { return true }
func (localModeWS) RoutesChannelEvents() bool  { return false }

func newHomeUI(busy, server bool) (*UI, *countingWorkspace) {
	ws := &countingWorkspace{ready: true, agentBusy: busy, sessions: makeSessions(2)}
	m := newPanelUI(ws)
	// Alt+W đọc trạng thái bận đã nhớ (không dò workspace trong Update).
	warmCaches(m, busy)
	if server {
		m.com.Workspace = serverModeWS{ws}
	} else {
		m.com.Workspace = localModeWS{ws}
	}
	return m, ws
}

func quitsHome(m *UI, cmd tea.Cmd) bool {
	for _, msg := range leafMsgs(cmd) {
		if _, ok := msg.(tea.QuitMsg); ok {
			return m.BackToHome()
		}
	}
	return false
}

func TestAltWMode(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		busy, server bool
		quitsAtOnce  bool
	}{
		"server, busy": {busy: true, server: true, quitsAtOnce: true},
		"server, idle": {busy: false, server: true, quitsAtOnce: true},
		"local, idle":  {busy: false, server: false, quitsAtOnce: true},
		"local, busy":  {busy: true, server: false, quitsAtOnce: false},
	} {
		m, ws := newHomeUI(tc.busy, tc.server)
		cmd := m.handleKeyPressMsg(altW)
		require.Equal(t, tc.quitsAtOnce, quitsHome(m, cmd), name)
		require.Equal(t, tc.quitsAtOnce, m.BackToHome(), name)
		require.Equal(t, !tc.quitsAtOnce, m.dialog.ContainsDialog(dialog.QuitID), "%s: confirmation dialog", name)
		require.Zero(t, ws.cancelCalls, "%s: Alt+W never cancels the run", name)
	}
}

// Hỏi xác nhận: Yep đưa về Home (và chỉ lúc đó mới đặt cờ), Nope/Esc ở lại.
func TestAltWConfirmationInLocalMode(t *testing.T) {
	t.Parallel()

	enter := tea.KeyPressMsg{Code: tea.KeyEnter}
	left := tea.KeyPressMsg{Code: tea.KeyLeft}

	m, _ := newHomeUI(true, false)
	m.handleKeyPressMsg(altW)
	require.True(t, m.dialog.ContainsDialog(dialog.QuitID))
	require.False(t, m.BackToHome(), "nothing happens until the user answers")

	// Mặc định là Nope.
	require.False(t, quitsHome(m, m.handleKeyPressMsg(enter)))
	require.False(t, m.dialog.HasDialogs())
	require.False(t, m.BackToHome())

	// Esc cũng ở lại.
	m.handleKeyPressMsg(altW)
	require.False(t, quitsHome(m, m.handleKeyPressMsg(esc)))
	require.False(t, m.dialog.HasDialogs())

	// Yep: về Home.
	m.handleKeyPressMsg(altW)
	m.handleKeyPressMsg(left)
	require.True(t, quitsHome(m, m.handleKeyPressMsg(enter)))
	require.True(t, m.BackToHome())
}

// Ctrl+C trong lúc đang hỏi về Home vẫn là thoát hẳn, không phải về Home.
func TestCtrlCDuringHomeConfirmationQuitsPlainly(t *testing.T) {
	t.Parallel()

	m, _ := newHomeUI(true, false)
	m.handleKeyPressMsg(altW)
	cmd := m.handleKeyPressMsg(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	var quit bool
	for _, msg := range leafMsgs(cmd) {
		_, ok := msg.(tea.QuitMsg)
		quit = quit || ok
	}
	require.True(t, quit)
	require.False(t, m.BackToHome())
}

// Mục "Home" của bảng lệnh chạy đúng việc của Alt+W, kể cả hỏi xác nhận ở chế độ
// local khi đang bận.
func TestHomeCommandActsLikeAltW(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		busy, server bool
		quitsAtOnce  bool
	}{
		"server, busy": {busy: true, server: true, quitsAtOnce: true},
		"local, idle":  {busy: false, server: false, quitsAtOnce: true},
		"local, busy":  {busy: true, server: false, quitsAtOnce: false},
	} {
		m, _ := newHomeUI(tc.busy, tc.server)
		m.dialog.OpenDialog(&actionDialog{action: dialog.ActionGoHome{}})
		cmds := m.handleDialogMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
		require.Equal(t, tc.quitsAtOnce, quitsHome(m, cmds), name)
		require.Equal(t, !tc.quitsAtOnce, m.dialog.ContainsDialog(dialog.QuitID), name)
	}
}
