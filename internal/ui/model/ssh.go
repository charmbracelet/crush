package model

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/sshaskpass"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/notification"
)

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
func (m *UI) handleSSHNotification(_ sshaskpass.Notification) {
	if m.dialog.ContainsDialog(dialog.SSHID) {
		m.dialog.CloseDialog(dialog.SSHID)
	}
}

// sshWaitingMessage describes the pending prompt for desktop
// notifications, phrased for the credential kind.
func sshWaitingMessage(req sshaskpass.PromptRequest) string {
	if req.Kind == sshaskpass.KindConfirm {
		return "SSH is waiting for you to confirm or touch your security key"
	}
	return "SSH needs your password to continue"
}

// sshWaitingNotification is the desktop notification shown alongside
// the SSH prompt dialog.
func sshWaitingNotification(req sshaskpass.PromptRequest) notification.Notification {
	return notification.Notification{
		Title:   "Crush is waiting...",
		Message: sshWaitingMessage(req),
	}
}
