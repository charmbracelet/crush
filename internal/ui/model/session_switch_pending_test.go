package model

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var errLoad = errors.New("load boom")

// Đích Alt+. nạp hỏng thì đích chờ không còn, và nạp treo quá lâu thì bị bỏ.
func TestCyclePendingClearedOnFailureAndExpires(t *testing.T) {
	t.Parallel()

	m, ws := newJumpUI()
	runLeaves(m.handleKeyPressMsg(altDot)) // Đích chờ: s2.
	m.Update(loadSessionFailedMsg{sessionID: "s2", err: errLoad})
	runLeaves(m.handleKeyPressMsg(altDot))
	require.Equal(t, []string{"s2", "s2"}, ws.loaded, "after a failed load the next press starts from the shown session")

	// Một lần nạp treo: sau thời hạn, lần bấm kế lại đi từ session đang xem.
	m, ws = newJumpUI()
	runLeaves(m.handleKeyPressMsg(altDot))
	m.cycle.at = time.Now().Add(-2 * cyclePendingTTL)
	runLeaves(m.handleKeyPressMsg(altDot))
	require.Equal(t, []string{"s2", "s2"}, ws.loaded)
}
