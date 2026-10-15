package model

// Alt+S không liệt kê session ngay trong Update (một vòng HTTP ở chế độ
// client/server): danh sách nạp bằng tea.Cmd rồi hộp thoại mở khi kết quả về
// (T5).

import (
	"context"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/util"
)

// pressAndLoad bấm một phím rồi đưa kết quả của các Cmd lá về Update, như vòng
// lặp Bubble Tea. Dùng cho phím mở hộp thoại Sessions, vốn mở khi danh sách về.
func pressAndLoad(m *UI, k tea.KeyPressMsg) {
	for _, msg := range leafMsgs(m.handleKeyPressMsg(k)) {
		if sd, ok := msg.(sessionsDialogMsg); ok {
			m.Update(sd)
		}
	}
}

func TestSessionsDialogOpensFromAsyncList(t *testing.T) {
	t.Parallel()

	ws := &countingWorkspace{ready: true, sessions: makeSessions(2)}
	m := newPanelUI(ws)
	start := time.Now()
	cmd := m.handleKeyPressMsg(altS)
	require.NotNil(t, cmd)
	require.Zero(t, ws.listCalls, "Update must not list sessions: that is an HTTP round-trip in client/server mode")
	require.False(t, m.dialog.ContainsDialog(dialog.SessionsID), "the dialog opens when the list arrives")
	require.Nil(t, m.handleKeyPressMsg(altS), "a second press while loading must not start another list")

	msg := cmd() // Chạy ngoài luồng Update khi chạy thật.
	require.Equal(t, 1, ws.listCalls)
	require.True(t, ws.listHadDeadline, "an unbounded list freezes the loading on a hung server")
	require.WithinDuration(t, start.Add(2*time.Second), ws.listDeadline, time.Second)

	_, next := m.Update(msg)
	require.Nil(t, next)
	require.True(t, m.dialog.ContainsDialog(dialog.SessionsID))
	require.Equal(t, 1, ws.listCalls, "building the dialog from the fetched list costs no further workspace call")

	// Đã mở rồi thì bấm lại chỉ đưa lên trước, không liệt kê nữa.
	require.Nil(t, m.handleKeyPressMsg(altS))
	require.Equal(t, 1, ws.listCalls)
}

// Liệt kê hỏng (máy chủ treo, quá hạn) thì báo cảnh báo thay vì mở hộp thoại, và
// lần bấm sau thử lại được.
func TestSessionsDialogListFailureWarns(t *testing.T) {
	t.Parallel()

	ws := &countingWorkspace{ready: true, listErr: context.DeadlineExceeded}
	m := newPanelUI(ws)
	cmd := m.handleKeyPressMsg(altS)
	require.NotNil(t, cmd)
	_, next := m.Update(cmd())
	require.False(t, m.dialog.ContainsDialog(dialog.SessionsID))
	require.NotNil(t, next)
	info, ok := next().(util.InfoMsg)
	require.True(t, ok)
	require.Equal(t, util.InfoTypeWarn, info.Type)
	require.Contains(t, info.Msg, "context deadline exceeded")

	ws.listErr = nil
	ws.sessions = makeSessions(2)
	pressAndLoad(m, altS)
	require.True(t, m.dialog.ContainsDialog(dialog.SessionsID), "the failed attempt does not wedge the loading flag")
}
