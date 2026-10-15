package dialog

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"
)

// pingMsg là message gửi cho đích danh một hộp thoại.
type pingMsg struct{ target string }

func (p pingMsg) TargetDialogID() string { return p.target }

// Message gửi đích danh đến đúng hộp thoại dù có hộp thoại khác mở đè: vòng
// làm tươi của Jobs không được chết khi một câu hỏi xin quyền bật lên.
func TestOverlay_TargetedMsgReachesDialogUnderneath(t *testing.T) {
	t.Parallel()

	under := &stubDialog{id: "under"}
	front := &stubDialog{id: "front"}
	o := NewOverlay(under, front)

	o.Update(pingMsg{target: "under"})
	require.Equal(t, []tea.Msg{pingMsg{target: "under"}}, under.received)
	require.Empty(t, front.received, "the dialog on top must not see a message addressed to another")

	o.Update(keyMsg('x'))
	require.Len(t, front.received, 1, "ordinary messages still go to the front dialog")
	require.Len(t, under.received, 1)
}

// Hộp thoại đích đã đóng thì message bị bỏ: vòng làm tươi tự dừng khi hộp
// thoại đóng, và message cũ không rơi vào hộp thoại khác.
func TestOverlay_TargetedMsgForClosedDialogIsDropped(t *testing.T) {
	t.Parallel()

	front := &stubDialog{id: "front"}
	o := NewOverlay(front)

	require.Nil(t, o.Update(pingMsg{target: "gone"}))
	require.Empty(t, front.received)

	require.Nil(t, NewOverlay().Update(pingMsg{target: "gone"}), "an empty overlay drops it too")
}
