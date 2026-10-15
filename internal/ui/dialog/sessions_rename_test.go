package dialog

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

// renameWorkspace ghi lại mọi lần đổi tên và mọi lần ghi cả hàng.
type renameWorkspace struct {
	workspace.Workspace

	renamed []string
	saved   []session.Session
}

func (w *renameWorkspace) RenameSession(_ context.Context, id, title string) error {
	w.renamed = append(w.renamed, id+"="+title)
	return nil
}

func (w *renameWorkspace) SaveSession(_ context.Context, s session.Session) (session.Session, error) {
	w.saved = append(w.saved, s)
	return s, nil
}

// Session nền vẫn chạy trong lúc dialog mở: đổi tên chỉ đổi tiêu đề (một lời
// gọi nguyên tử), không ghi lại cả hàng bằng bản chụp lúc mở dialog, kẻo đè
// cost/token/todos/summary mà run nền vừa ghi.
func TestRenameSessionChangesOnlyTheTitle(t *testing.T) {
	t.Parallel()

	ws := &renameWorkspace{}
	d := &Session{com: &common.Common{Workspace: ws}}
	stale := session.Session{ID: "s1", Title: "new", Cost: 0.1, PromptTokens: 10}

	require.Nil(t, d.updateSessionCmd(stale)())
	require.Equal(t, []string{"s1=new"}, ws.renamed)
	require.Empty(t, ws.saved, "renaming must not rewrite the whole row")
}
