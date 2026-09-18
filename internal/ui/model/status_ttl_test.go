package model

import (
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/ui/util"
	"github.com/stretchr/testify/require"
)

// TestAMessageOutlivesAnEarlierOnesTimer covers the bug that made a status
// message look like it vanished early. Each one schedules its own expiry,
// and the timers said nothing about which message they were for, so the
// first to fire retired whatever happened to be on screen. A message that
// replaced an earlier one therefore served out the remainder of its
// predecessor's time instead of its own.
//
// It showed up worst on anything reached through a slower interaction,
// where a message was more likely to already be counting down: the same
// notice looked long-lived when it arrived on its own and clipped when it
// followed something else.
func TestAMessageOutlivesAnEarlierOnesTimer(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true}
	m := newBusyUI(ws)

	m.Update(util.InfoMsg{Type: util.InfoTypeInfo, Msg: "something earlier"})
	staleSeq := m.status.msgSeq

	m.Update(util.InfoMsg{Type: util.InfoTypeSuccess, Msg: "what the user is here to read"})

	// The earlier message's timer comes due. It must leave the newer one be.
	m.Update(util.ClearStatusMsg{Seq: staleSeq})
	require.False(t, m.status.msg.IsEmpty(), "a stale timer must not retire a newer message")
	require.Equal(t, "what the user is here to read", m.status.msg.Msg)

	// Its own timer still retires it.
	m.Update(util.ClearStatusMsg{Seq: m.status.msgSeq})
	require.True(t, m.status.msg.IsEmpty(), "a message still expires on its own timer")
}

// TestEachMessageIsNamedByItsOwnTimer pins the mechanism directly: every
// message gets a fresh sequence, and only the matching one clears it.
func TestEachMessageIsNamedByItsOwnTimer(t *testing.T) {
	pinTTLs(t)

	s := newBusyUI(&countingWorkspace{ready: true}).status

	first := s.SetInfoMsg(util.InfoMsg{Type: util.InfoTypeInfo, Msg: "first"})
	second := s.SetInfoMsg(util.InfoMsg{Type: util.InfoTypeInfo, Msg: "second"})
	require.NotEqual(t, first, second, "each message needs its own name")

	require.False(t, s.ClearInfoMsg(first), "the first timer no longer owns the screen")
	require.Equal(t, "second", s.msg.Msg)

	require.True(t, s.ClearInfoMsg(second), "the owning timer clears it")
	require.True(t, s.msg.IsEmpty())
}

// TestAnExpiredTimerDoesNotClearALaterRepeat guards the case where the same
// text is shown twice. The second showing is a different message and keeps
// its own time, even though it reads identically.
func TestAnExpiredTimerDoesNotClearALaterRepeat(t *testing.T) {
	pinTTLs(t)

	s := newBusyUI(&countingWorkspace{ready: true}).status
	same := util.InfoMsg{Type: util.InfoTypeWarn, Msg: "same words", TTL: time.Second}

	first := s.SetInfoMsg(same)
	require.True(t, s.ClearInfoMsg(first))

	second := s.SetInfoMsg(same)
	require.False(t, s.ClearInfoMsg(first), "the old timer must not reach the repeat")
	require.False(t, s.msg.IsEmpty())
	require.True(t, s.ClearInfoMsg(second))
}
