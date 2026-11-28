package model

import (
	"time"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/common"
)

// pendingTurn is the turn running in a session, sent by this client or
// adopted from the busy probe. It lives until the run's RunComplete (or a
// failure to start it) arrives.
type pendingTurn struct {
	// sessionID is the session running the turn; empty when none.
	sessionID string
	// echo shows the prompt until the stored user message replaces it.
	echo chat.MessageItem
	// after is the last prompt stored before this turn's; empty if none.
	after string
	// spinner is the turn timer at the tail, kept so moving it keeps its
	// animation.
	spinner *chat.PendingAssistantItem
	// carrier is the item whose spinner shows the turn time in place of the
	// timer item.
	carrier turnTimerCarrier
	// adopted marks a turn this client did not send, picked up from the
	// busy probe, so an idle probe may end it.
	adopted bool
}

// adoptRunningTurn times a turn already running in the current session that
// this client did not send: one started elsewhere, or before the session was
// opened. The clock counts from the last prompt, when the turn began.
func (m *UI) adoptRunningTurn(sessionID string) {
	if sessionID == "" || sessionID != m.currentSessionID() || m.pending.sessionID == sessionID {
		return
	}
	// A turn this client sent in another session keeps its own state.
	if m.pending.sessionID != "" && !m.pending.adopted {
		return
	}
	start := time.Now()
	if m.lastUserMessageTime > 0 {
		start = time.Unix(m.lastUserMessageTime, 0)
	}
	common.StartTurnAt(sessionID, start)
	m.pending = pendingTurn{sessionID: sessionID, adopted: true}
}

// turnTimerCarrier is an item whose spinner can show the turn time: an
// assistant message before its first output, a call whose arguments are
// streaming, or a working subagent.
type turnTimerCarrier interface {
	chat.MessageItem
	CarriesTurnTimer() bool
	SetTurnTimer(sessionID string)
}

// setCarrier moves the turn time onto item, or off every item when nil.
func (m *UI) setCarrier(item turnTimerCarrier) {
	if m.pending.carrier == item {
		return
	}
	if m.pending.carrier != nil {
		m.pending.carrier.SetTurnTimer("")
	}
	m.pending.carrier = item
	if item != nil {
		item.SetTurnTimer(m.pending.sessionID)
	}
}

// turnTail describes where the turn time shows, from what the tail is doing.
type turnTail struct {
	// carrier is the spinning item showing the time; the timer item hides.
	carrier turnTimerCarrier
	// running: a tool is executing and nothing streams, so the timer item
	// is the static "Running" line rather than a spinner.
	running bool
}

func (m *UI) turnTail() turnTail {
	item := m.chat.lastItemBeforeTurnTimer()
	if c, ok := item.(turnTimerCarrier); ok && c.CarriesTurnTimer() {
		return turnTail{carrier: c}
	}
	_, isTool := item.(chat.ToolMessageItem)
	return turnTail{running: isTool}
}

// showPendingTurn shows the prompt and the turn spinner for sessionID,
// ahead of the agent storing the message and publishing it back. Reports
// whether a placeholder was added.
func (m *UI) showPendingTurn(sessionID, prompt string, attachments []message.Attachment) bool {
	m.pending = pendingTurn{sessionID: sessionID, after: m.chat.LastPromptID()}
	if prompt != "" || len(attachments) > 0 {
		m.pending.echo = chat.NewPendingUserItem(m.com.Styles, prompt, attachments)
	}
	return m.syncPendingItem()
}

// endPendingTurn retires the pending items, the clock and the busy probe for
// sessionID, if that is the session currently running a turn.
func (m *UI) endPendingTurn(sessionID string) {
	common.StopTurn(sessionID)
	// A run ending is a busy edge: drop the probe and any still in flight.
	m.invalidateBusyCaches()
	if m.probedSession == sessionID {
		m.probedSessionBusy = false
	}
	if m.pending.sessionID == sessionID {
		m.setCarrier(nil)
		m.pending = pendingTurn{}
		m.syncPendingItem()
	}
}

// turnInFlight reports whether a prompt sent to sessionID now would queue
// behind a running turn rather than start one.
func (m *UI) turnInFlight(sessionID string) bool {
	return m.pending.sessionID == sessionID ||
		m.probedSession == sessionID && m.probedSessionBusy
}

// syncPendingItem makes the chat match the pending turn, adding or removing
// the prompt echo and the placeholder spinner as needed, and reports whether
// it added the spinner. It is idempotent, so callers may run it on any path
// that could have changed either the pending turn or the items in view.
func (m *UI) syncPendingItem() bool {
	if m.pending.sessionID == "" || m.pending.sessionID != m.currentSessionID() {
		m.setCarrier(nil)
		m.chat.RemoveMessage(chat.PendingUserID)
		m.chat.RemoveMessage(chat.PendingAssistantID)
		return false
	}
	// The echo stands in for the prompt until a newer stored one shows up.
	if m.pending.echo != nil && m.chat.LastPromptID() != m.pending.after {
		m.pending.echo = nil
		m.chat.RemoveMessage(chat.PendingUserID)
	}
	if m.pending.echo != nil && m.chat.MessageItem(chat.PendingUserID) == nil {
		m.chat.AppendMessages(m.pending.echo)
	}
	// The timer stays last: hidden while something spinning shows the time,
	// a spinner while tokens stream or are awaited, "Running" while a tool runs.
	tail := m.turnTail()
	m.setCarrier(tail.carrier)
	if tail.carrier != nil {
		m.chat.RemoveMessage(chat.PendingAssistantID)
		return false
	}
	if m.pending.spinner == nil {
		m.pending.spinner = chat.NewPendingAssistantItem(m.com.Styles, m.pending.sessionID)
	}
	m.pending.spinner.SetLive(!tail.running)
	if m.chat.MessageItem(chat.PendingAssistantID) != nil {
		if m.chat.MessageItemAt(m.chat.Len()-1) == m.pending.spinner {
			return false
		}
		m.chat.RemoveMessage(chat.PendingAssistantID)
	}
	m.chat.AppendMessages(m.pending.spinner)
	return true
}
