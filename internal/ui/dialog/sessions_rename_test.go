package dialog

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

// renameWorkspace trả session "mới nhất" từ GetSession và ghi lại mọi lần lưu.
type renameWorkspace struct {
	workspace.Workspace

	fresh session.Session
	saved []session.Session
}

func (w *renameWorkspace) GetSession(context.Context, string) (session.Session, error) {
	return w.fresh, nil
}

func (w *renameWorkspace) SaveSession(_ context.Context, s session.Session) (session.Session, error) {
	w.saved = append(w.saved, s)
	return s, nil
}

// Session nền vẫn chạy trong lúc dialog mở: đổi tên chỉ được đổi tiêu đề,
// không được ghi đè cost/token/todos/summary bằng bản chụp lúc mở dialog.
func TestRenameSessionKeepsFreshState(t *testing.T) {
	t.Parallel()

	ws := &renameWorkspace{fresh: session.Session{
		ID: "s1", Title: "old", Cost: 2.5, PromptTokens: 900,
		SummaryMessageID: "m9", Todos: []session.Todo{{Content: "ship"}},
	}}
	d := &Session{com: &common.Common{Workspace: ws}}
	stale := session.Session{ID: "s1", Title: "new", Cost: 0.1, PromptTokens: 10}

	require.Nil(t, d.updateSessionCmd(stale)())
	require.Len(t, ws.saved, 1)
	got := ws.saved[0]
	require.Equal(t, "new", got.Title)
	require.Equal(t, 2.5, got.Cost)
	require.EqualValues(t, 900, got.PromptTokens)
	require.Equal(t, "m9", got.SummaryMessageID)
	require.Len(t, got.Todos, 1)
}
