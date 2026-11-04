package model

import (
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/common"
)

// pendingTurn is a turn this client sent that has not yet ended. It lives
// until the run's RunComplete (or a failure to start it) arrives.
type pendingTurn struct {
	// sessionID is the session running the turn; empty when none.
	sessionID string
	// echo shows the prompt until the stored user message replaces it.
	echo chat.MessageItem
	// after is the last prompt stored before this turn's; empty if none.
	after string
	// spinner is the placeholder, kept so moving it keeps its animation.
	spinner chat.MessageItem
	// handedOff records that an assistant message now carries the spinner.
	handedOff bool
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

// handOffPendingTurn drops the placeholder for the assistant message that
// now carries the spinner, without ending the turn.
func (m *UI) handOffPendingTurn() {
	m.pending.handedOff = true
	m.pending.spinner = nil
	m.syncPendingItem()
}

// endPendingTurn retires the pending items and the clock for sessionID, if
// that is the session currently running a turn.
func (m *UI) endPendingTurn(sessionID string) {
	common.StopTurn(sessionID)
	if m.pending.sessionID == sessionID {
		m.pending = pendingTurn{}
		m.syncPendingItem()
	}
}

// turnInFlight reports whether a prompt sent to sessionID now would queue
// behind a running turn rather than start one.
func (m *UI) turnInFlight(sessionID string) bool {
	if m.pending.sessionID == sessionID {
		return true
	}
	if m.probedSession == sessionID && m.probedSessionBusy {
		return true
	}
	return m.chat.HasLiveSpinner()
}

// syncPendingItem makes the chat match the pending turn, adding or removing
// the prompt echo and the placeholder spinner as needed, and reports whether
// it added the spinner. It is idempotent, so callers may run it on any path
// that could have changed either the pending turn or the items in view.
func (m *UI) syncPendingItem() bool {
	if m.pending.sessionID == "" || m.pending.sessionID != m.currentSessionID() {
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
	if m.pending.handedOff {
		m.chat.RemoveMessage(chat.PendingAssistantID)
		return false
	}
	if m.chat.MessageItem(chat.PendingAssistantID) != nil {
		return false
	}
	if m.pending.spinner == nil {
		m.pending.spinner = chat.NewPendingAssistantItem(m.com.Styles, m.pending.sessionID)
	}
	m.chat.AppendMessages(m.pending.spinner)
	return true
}
