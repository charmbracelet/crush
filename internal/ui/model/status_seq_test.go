package model

import (
	"testing"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/util"
	"github.com/stretchr/testify/require"
)

// A delayed clear carries the generation of the message it was
// scheduled for — a stale tick must not wipe a newer message (the
// persistent stall warning included).
func TestStatusStaleClearKeepsNewerMessage(t *testing.T) {
	t.Parallel()

	m := newPrismTestUI()
	m.status.SetInfoMsg(util.InfoMsg{Msg: "first"})
	seqA := m.status.MsgSeq()
	m.status.SetInfoMsg(util.InfoMsg{Msg: "second"})

	// The tick scheduled for "first" fires late — "second" survives.
	m.Update(util.ClearStatusMsg{Seq: seqA})
	require.Equal(t, "second", m.status.InfoMsg().Msg)

	// The tick scheduled for "second" clears it.
	m.Update(util.ClearStatusMsg{Seq: m.status.MsgSeq()})
	require.True(t, m.status.InfoMsg().IsEmpty())
}

// A stall resolve clears only the warning its own session raised, and
// only while that warning is still the displayed message.
func TestNotebookStallResolveScopedToSessionAndMessage(t *testing.T) {
	t.Parallel()

	m := newPrismTestUI()
	m.handleAgentNotification(notify.Notification{
		Type: notify.TypeNotebookStall, SessionID: "s1", Message: "stalled",
	})
	require.Equal(t, "stalled", m.status.InfoMsg().Msg)

	// A newer unrelated message replaced the warning — the resolve
	// must not clear it.
	m.status.SetInfoMsg(util.InfoMsg{Msg: "unrelated"})
	m.handleAgentNotification(notify.Notification{
		Type: notify.TypeNotebookStallResolved, SessionID: "s1",
	})
	require.Equal(t, "unrelated", m.status.InfoMsg().Msg)

	// A resolve from a different session leaves the warning alone.
	m.handleAgentNotification(notify.Notification{
		Type: notify.TypeNotebookStall, SessionID: "s1", Message: "stalled",
	})
	m.handleAgentNotification(notify.Notification{
		Type: notify.TypeNotebookStallResolved, SessionID: "s2",
	})
	require.Equal(t, "stalled", m.status.InfoMsg().Msg)

	// The matching resolve clears it.
	m.handleAgentNotification(notify.Notification{
		Type: notify.TypeNotebookStallResolved, SessionID: "s1",
	})
	require.True(t, m.status.InfoMsg().IsEmpty())
}

// A session deleted mid-stall never emits a resolve — its warn record
// must not linger in the model.
func TestNotebookStallWarnDroppedOnSessionDelete(t *testing.T) {
	t.Parallel()

	m := newPrismTestUI()
	// The stalled session is not the current one — deleting the
	// current session would route into newSession, which this
	// minimal model does not wire.
	m.session.ID = "s9"
	m.handleAgentNotification(notify.Notification{
		Type: notify.TypeNotebookStall, SessionID: "s1", Message: "stalled",
	})
	require.Contains(t, m.nbStallWarned, "s1")

	m.Update(pubsub.Event[session.Session]{
		Type:    pubsub.DeletedEvent,
		Payload: session.Session{ID: "s1"},
	})
	require.NotContains(t, m.nbStallWarned, "s1")
}
