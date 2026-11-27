package model

import (
	"log/slog"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/pinentry"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/notification"
	"github.com/charmbracelet/crush/internal/ui/util"
)

// pinentryTouchMessage is shown in the status bar while gpg waits for a
// security key touch after the passphrase dialog closed.
const pinentryTouchMessage = "GPG is waiting: touch your security key to continue"

// handlePinentryEvent applies a pinentry watcher event: it hands the
// terminal over while a terminal-based pinentry dialog is up, takes it
// back once the dialog closes, and surfaces a status hint while gpg waits
// for a security key touch.
func (m *UI) handlePinentryEvent(ev pinentry.Event) tea.Cmd {
	var cmds []tea.Cmd

	if ev.Active != m.pinentryActive {
		m.pinentryActive = ev.Active
		m.pinentryCtrlL = ev.CtrlLRedraw
		if ev.Active {
			if cmd := m.releaseTerminalForPinentry(); cmd != nil {
				cmds = append(cmds, cmd)
			}
			if cmd := m.sendNotification(notification.Notification{
				Title:   "Crush is waiting...",
				Message: "GPG is asking for your passphrase (pinentry)",
			}); cmd != nil {
				cmds = append(cmds, cmd)
			}
		} else if cmd := m.restoreTerminalForPinentry(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	if ev.TouchPending != m.pinentryTouchPending {
		m.pinentryTouchPending = ev.TouchPending
		if ev.TouchPending {
			cmds = append(cmds, m.status.Show(util.InfoMsg{
				Type: util.InfoTypeWarn,
				Msg:  pinentryTouchMessage,
			}))
		} else {
			m.status.ClearInfoMsg(m.status.msgSeq)
		}
	}

	return tea.Batch(cmds...)
}

// releaseTerminalForPinentry releases the terminal so a pinentry dialog
// spawned by gpg-agent can draw on it and read input directly.
func (m *UI) releaseTerminalForPinentry() tea.Cmd {
	program := m.program
	if program == nil {
		return nil
	}
	ctrlL := m.pinentryCtrlL
	return func() tea.Msg {
		// Bubble Tea runs each command in its own goroutine; serialize the
		// release/restore pair so a restore can never overtake a release.
		m.pinentryTermMu.Lock()
		defer m.pinentryTermMu.Unlock()
		if err := program.ReleaseTerminal(); err != nil {
			slog.Error("Failed to release terminal for pinentry", "error", err)
			return nil
		}
		pinentry.BeginHandover(ctrlL)
		return nil
	}
}

// restoreTerminalForPinentry reinitializes the renderer and input reader
// after a pinentry dialog closed, redrawing the Crush UI.
func (m *UI) restoreTerminalForPinentry() tea.Cmd {
	program := m.program
	if program == nil {
		return nil
	}
	return func() tea.Msg {
		m.pinentryTermMu.Lock()
		defer m.pinentryTermMu.Unlock()
		pinentry.EndHandover()
		if err := program.RestoreTerminal(); err != nil {
			slog.Error("Failed to restore terminal after pinentry", "error", err)
		}
		return nil
	}
}

// openPinentryDialog shows the integrated pinentry dialog for a GPG
// credential request (passphrase or security key PIN). Input is masked;
// the secret is returned via [dialog.ActionPinentrySubmit].
func (m *UI) openPinentryDialog(req pinentry.PromptRequest) tea.Cmd {
	// Close any existing pinentry dialog first to prevent stacking.
	m.dialog.CloseDialog(dialog.PinentryID)
	m.dialog.OpenDialogWithGrace(dialog.NewPinentry(m.com, req))

	what := "passphrase"
	if req.Kind == pinentry.KindPIN {
		what = "security key PIN"
	}
	cmds := []tea.Cmd{m.sendNotification(notification.Notification{
		Title:   "Crush is waiting...",
		Message: "GPG needs your " + what + " to continue",
	})}
	// A retry carries why GPG rejected the previous attempt; surface it
	// as a caution-level status message rather than inside the dialog.
	if req.Error != "" {
		cmds = append(cmds, util.ReportWarn(req.Error))
	}
	return tea.Batch(cmds...)
}

// handlePinentryNotification dismisses the pinentry dialog once its
// prompt is resolved, covering the case where another subscriber (or a
// cancelled GPG command) resolved it first. A newer prompt's dialog is
// left alone: notifications can arrive after the next prompt opened.
func (m *UI) handlePinentryNotification(n pinentry.Notification) {
	if d, ok := m.dialog.Dialog(dialog.PinentryID).(*dialog.Pinentry); ok && d.RequestID() == n.RequestID {
		m.dialog.CloseDialog(dialog.PinentryID)
	}
}
