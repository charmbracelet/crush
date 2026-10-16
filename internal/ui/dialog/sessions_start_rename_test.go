package dialog

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// StartRename (F2 ở màn chat) vào thẳng chế độ đổi tên cho đúng session đang
// mở, không phải session đầu danh sách; id lạ thì từ chối và giữ chế độ thường.
func TestStartRenameTargetsGivenSession(t *testing.T) {
	t.Parallel()

	base := newDeleteDialog(t, &busyProbeWorkspace{sessions: twoSessions()})
	// Dựng lại với s2 được chọn, như lúc mở từ phiên đang ở s2.
	d := NewSessionsFromList(base.com, "s2", twoSessions())

	require.False(t, d.StartRename("nope"))
	require.Equal(t, sessionsModeNormal, d.sessionsMode)

	require.True(t, d.StartRename("s2"))
	require.Equal(t, sessionsModeUpdating, d.sessionsMode)
	require.Equal(t, "s2", d.selectedSessionItem().ID())
}
