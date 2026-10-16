package model

// F2 ở màn chat đổi tên session hiện tại bằng đúng hộp thoại Sessions ở chế độ
// đổi tên (ctrl+r trong hộp thoại), không có hộp thoại riêng.

import (
	"context"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/dialog"
)

var f2 = tea.KeyPressMsg{Code: tea.KeyF2}

// renameRecorder ghi lại các lần RenameSession; còn lại dùng countingWorkspace.
type renameRecorder struct {
	*countingWorkspace

	mu      sync.Mutex
	renamed []string
}

func (w *renameRecorder) RenameSession(_ context.Context, id, title string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.renamed = append(w.renamed, id+"="+title)
	return nil
}

func TestF2RenamesCurrentSession(t *testing.T) {
	t.Parallel()

	for _, focus := range []uiFocusState{uiFocusEditor, uiFocusMain} {
		base := &countingWorkspace{ready: true, sessions: makeSessions(3)}
		ws := &renameRecorder{countingWorkspace: base}
		m := newPanelUI(base)
		m.com.Workspace = ws
		m.session = &session.Session{ID: "s2", Title: "title 2"} // Không phải session đầu danh sách.
		m.focus = focus

		pressAndLoad(m, f2)
		require.True(t, m.dialog.ContainsDialog(dialog.SessionsID), "focus %d", focus)

		// Gõ thêm một ký tự vào ô đổi tên rồi Enter: lệnh ghi đổi tên phải nhắm s2.
		m.handleKeyPressMsg(tea.KeyPressMsg{Code: 'x', Text: "x"})
		leafMsgs(m.handleKeyPressMsg(tea.KeyPressMsg{Code: tea.KeyEnter}))
		require.Equal(t, []string{"s2=title 2x"}, ws.renamed, "focus %d", focus)
	}
}

func TestF2WithoutSessionDoesNothing(t *testing.T) {
	t.Parallel()

	ws := &countingWorkspace{ready: true, sessions: makeSessions(2)}
	m := newPanelUI(ws)
	m.session = nil

	require.NotPanics(t, func() { require.Nil(t, m.handleKeyPressMsg(f2)) })
	require.False(t, m.dialog.HasDialogs())
	require.Zero(t, ws.listCalls, "no session: F2 must not even list")
}

// Rename xong, workspace phát sự kiện session cập nhật: UI áp tiêu đề mới.
func TestRenamedTitleIsApplied(t *testing.T) {
	t.Parallel()

	m := newPanelUI(&countingWorkspace{ready: true, sessions: makeSessions(2)})
	m.session = &session.Session{ID: "s1", Title: "old"}
	m.Update(pubsub.Event[session.Session]{Type: pubsub.UpdatedEvent, Payload: session.Session{ID: "s1", Title: "new"}})
	require.Equal(t, "new", m.session.Title)
}
