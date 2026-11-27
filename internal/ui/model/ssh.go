package model

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/crush/internal/sshaskpass"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/notification"
	"github.com/charmbracelet/crush/internal/ui/util"
)

// sshTouchWarningTTL bounds how long the security key touch warning
// stays up. It normally clears as soon as the touch lands; this is only
// a backstop matching how long the prompt server holds a request.
const sshTouchWarningTTL = 10 * time.Minute

// handleSSHPrompt routes an SSH askpass request: a passive security key
// touch is surfaced as a warning until it lands, while
// passwords and decision questions open the dialog.
func (m *UI) handleSSHPrompt(req sshaskpass.PromptRequest) []tea.Cmd {
	var cmds []tea.Cmd

	req = sshaskpass.Describe(req)

	if req.Kind == sshaskpass.KindTouch {
		// The physical touch is the approval, so there is nothing to
		// answer: show a warning until the prompt server reports the
		// touch landed (see handleSSHNotification).
		cmds = append(cmds, m.status.Show(util.InfoMsg{
			Type: util.InfoTypeWarn,
			Msg:  req.Message,
			TTL:  sshTouchWarningTTL,
		}))
		m.sshTouchID, m.sshTouchSeq = req.ID, m.status.msgSeq
		if cmd := m.sendNotification(notification.Notification{
			Title:   "Crush is waiting...",
			Message: req.Message,
		}); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return cmds
	}

	if cmd := m.openSSHDialog(req); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if cmd := m.sendNotification(sshWaitingNotification(req)); cmd != nil {
		cmds = append(cmds, cmd)
	}
	return cmds
}

// openSSHDialog shows the integrated SSH askpass dialog for a credential
// or confirmation request. Input is masked; the value is returned via
// [dialog.ActionSSHSubmit].
func (m *UI) openSSHDialog(req sshaskpass.PromptRequest) tea.Cmd {
	// Close any existing SSH dialog first to prevent stacking.
	m.dialog.CloseDialog(dialog.SSHID)
	m.dialog.OpenDialogWithGrace(dialog.NewSSH(m.com, req))
	return nil
}

// handleSSHNotification dismisses the SSH dialog once the prompt is
// resolved, covering the case where another subscriber (or a cancelled
// ssh command) resolved it first.
func (m *UI) handleSSHNotification(n sshaskpass.Notification) {
	if n.RequestID != "" && n.RequestID == m.sshTouchID {
		m.status.ClearInfoMsg(m.sshTouchSeq)
		m.sshTouchID = ""
	}
	if d, ok := m.dialog.Dialog(dialog.SSHID).(*dialog.SSH); ok && d.RequestID() == n.RequestID {
		m.dialog.CloseDialog(dialog.SSHID)
	}
}

// sshWaitingMessage describes the pending prompt for desktop
// notifications, phrased for the credential kind.
func sshWaitingMessage(req sshaskpass.PromptRequest) string {
	if req.Message != "" {
		return req.Message
	}
	switch req.Kind {
	case sshaskpass.KindConfirm:
		return "SSH is waiting for you to confirm the security key or host"
	case sshaskpass.KindTouch:
		return "Touch your security key to continue"
	default:
		return "SSH needs your password to continue"
	}
}

// sshWaitingNotification is the desktop notification shown alongside
// the SSH prompt dialog.
func sshWaitingNotification(req sshaskpass.PromptRequest) notification.Notification {
	return notification.Notification{
		Title:   "Crush is waiting...",
		Message: sshWaitingMessage(req),
	}
}
