package model

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/util"
	"github.com/stretchr/testify/require"
)

// expire runs a status expiry command and returns the message it delivers.
func expire(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	require.NotNil(t, cmd)
	return cmd()
}

func instant(text string) util.InfoMsg {
	return util.InfoMsg{Type: util.InfoTypeInfo, Msg: text, TTL: time.Millisecond}
}

// TestStatusMessageKeepsItsOwnTime checks that each message is retired by
// its own timer only, including when the same text is shown twice.
func TestStatusMessageKeepsItsOwnTime(t *testing.T) {
	t.Parallel()

	s := NewStatus(common.DefaultCommon(&countingWorkspace{ready: true}), nil)

	first := expire(t, s.Show(instant("same words"))).(util.ClearStatusMsg)
	second := expire(t, s.Show(instant("same words"))).(util.ClearStatusMsg)

	s.ClearInfoMsg(first.Seq)
	require.Equal(t, "same words", s.msg.Msg, "a stale timer must not retire a newer message")

	s.ClearInfoMsg(second.Seq)
	require.True(t, s.msg.IsEmpty(), "a message still expires on its own timer")
}

// TestStaleStatusTimerThroughUpdate covers the same rule on the path the
// UI actually takes: a message reported through Update outlives the timer
// of the one it replaced.
func TestStaleStatusTimerThroughUpdate(t *testing.T) {
	pinTTLs(t)

	m := newBusyUI(&countingWorkspace{ready: true})
	stale := expire(t, m.status.Show(instant("something earlier")))

	m.Update(util.InfoMsg{Type: util.InfoTypeSuccess, Msg: "what the user is here to read"})
	m.Update(stale)

	require.Equal(t, "what the user is here to read", m.status.msg.Msg)
}
