package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Footer của hộp thoại khởi tạo dự án không được lặp "ctrl+c quit": phần chung
// của ShortHelp đã thêm Quit cho mọi trạng thái.
func TestInitializeShortHelpListsQuitOnce(t *testing.T) {
	t.Parallel()
	m := newBusyUI(&countingWorkspace{ready: true})
	m.state = uiInitialize
	var quits int
	for _, b := range m.ShortHelp() {
		if b.Help().Desc == m.keyMap.Quit.Help().Desc {
			quits++
		}
	}
	require.Equal(t, 1, quits)
}
