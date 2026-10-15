package dialog

import (
	"context"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/ui/util"
	"github.com/charmbracelet/crush/internal/workspace"
)

// busyProbeWorkspace đếm các lần dò "agent có sẵn sàng / session có đang chạy":
// ở chế độ client/server mỗi lần là một vòng HTTP.
type busyProbeWorkspace struct {
	workspace.Workspace

	sessions []session.Session
	ready    bool
	busy     bool

	mu     sync.Mutex
	probes int
}

func (w *busyProbeWorkspace) ListSessions(context.Context) ([]session.Session, error) {
	return w.sessions, nil
}

func (w *busyProbeWorkspace) AgentIsReady() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.probes++
	return w.ready
}

func (w *busyProbeWorkspace) AgentIsSessionBusy(string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.probes++
	return w.busy
}

func (w *busyProbeWorkspace) probeCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.probes
}

func newDeleteDialog(t *testing.T, ws *busyProbeWorkspace) *Session {
	t.Helper()
	sty := styles.CharmtonePantera()
	d, err := NewSessions(&common.Common{Workspace: ws, Styles: &sty}, "s1")
	require.NoError(t, err)
	return d
}

var ctrlX = tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl}

func twoSessions() []session.Session {
	return []session.Session{{ID: "s1", Title: "one"}, {ID: "s2", Title: "two"}}
}

// Ctrl+X không dò workspace ngay trong HandleMsg (chạy trên luồng Update): việc
// dò là một Cmd, và kết quả của nó quyết định vào chế độ xoá hay từ chối.
func TestSessionsDeleteProbesBusyOffThread(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		ready, busy bool
		wantDelete  bool
	}{
		"idle session":    {ready: true, busy: false, wantDelete: true},
		"running session": {ready: true, busy: true, wantDelete: false},
		"agent not ready": {ready: false, busy: true, wantDelete: true},
	} {
		ws := &busyProbeWorkspace{sessions: twoSessions(), ready: tc.ready, busy: tc.busy}
		d := newDeleteDialog(t, ws)

		act := d.HandleMsg(ctrlX)
		require.Zero(t, ws.probeCount(), "%s: HandleMsg must not probe the workspace", name)
		require.Equal(t, sessionsModeNormal, d.sessionsMode, name)
		cmd, ok := act.(ActionCmd)
		require.True(t, ok, name)
		require.NotNil(t, cmd.Cmd, name)

		msg := cmd.Cmd()
		require.Positive(t, ws.probeCount(), "%s: the probe runs in the command", name)

		res := d.HandleMsg(msg)
		if tc.wantDelete {
			require.Nil(t, res, name)
			require.Equal(t, sessionsModeDeleting, d.sessionsMode, name)
			continue
		}
		require.Equal(t, sessionsModeNormal, d.sessionsMode, name)
		warn, ok := res.(ActionCmd)
		require.True(t, ok, name)
		info, ok := warn.Cmd().(util.InfoMsg)
		require.True(t, ok, name)
		require.Equal(t, util.InfoTypeWarn, info.Type)
		require.Contains(t, info.Msg, "busy")
	}
}

// Người dùng chuyển mục chọn trong lúc đang dò: kết quả của mục cũ không được
// kéo hộp thoại vào chế độ hỏi xoá mục mới.
func TestSessionsDeleteIgnoresProbeForOldSelection(t *testing.T) {
	t.Parallel()

	ws := &busyProbeWorkspace{sessions: twoSessions(), ready: true}
	d := newDeleteDialog(t, ws)
	require.Equal(t, "s1", d.selectedSessionItem().ID())

	cmd := d.HandleMsg(ctrlX).(ActionCmd).Cmd
	d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Equal(t, "s2", d.selectedSessionItem().ID())

	require.Nil(t, d.HandleMsg(cmd()))
	require.Equal(t, sessionsModeNormal, d.sessionsMode, "the probe was for s1, the selection is s2")
}

// Danh sách rỗng: Ctrl+X vào thẳng chế độ xoá như trước, không có gì để dò.
func TestSessionsDeleteWithNothingSelected(t *testing.T) {
	t.Parallel()

	ws := &busyProbeWorkspace{ready: true, busy: true}
	d := newDeleteDialog(t, ws)
	require.Nil(t, d.HandleMsg(ctrlX))
	require.Equal(t, sessionsModeDeleting, d.sessionsMode)
	require.Zero(t, ws.probeCount())
}

// NewSessionsFromList dựng hộp thoại từ danh sách có sẵn, không gọi workspace.
func TestNewSessionsFromListDoesNoIO(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	d := NewSessionsFromList(&common.Common{Styles: &sty}, "s2", twoSessions())
	require.Equal(t, "s2", d.selectedSessionItem().ID(), "the viewed session is preselected")
}
