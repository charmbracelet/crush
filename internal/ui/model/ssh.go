package model

import (
	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/crush/internal/sshaskpass"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/notification"
	"github.com/charmbracelet/crush/internal/ui/util"
)

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
		cmds = append(cmds, m.status.ShowKeyed(sshTouchKey(req.ID), util.InfoMsg{
			Type: util.InfoTypeWarn,
			Msg:  req.Message,
		}))
		if cmd := m.sendNotification(sshWaitingNotification(req)); cmd != nil {
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
	m.status.Dismiss(sshTouchKey(n.RequestID))
	if d, ok := m.dialog.Dialog(dialog.SSHID).(*dialog.SSH); ok && d.RequestID() == n.RequestID {
		m.dialog.CloseDialog(dialog.SSHID)
	}
}

// sshTouchKey keys the status warning for a pending security key touch.
func sshTouchKey(requestID string) string {
	return "ssh-touch:" + requestID
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
