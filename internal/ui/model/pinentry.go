package model

import (
	"fmt"
	"log/slog"
	"os"

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
			m.status.SetInfoMsg(util.InfoMsg{
				Type: util.InfoTypeWarn,
				Msg:  pinentryTouchMessage,
			})
		} else {
			m.status.ClearInfoMsg()
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
		// The release restored the pre-Crush terminal modes (canonical,
		// echo), clobbering the modes the pinentry dialog configured when
		// it started. Re-apply raw/noecho so the dialog's input is not
		// line-buffered or echoed in cleartext. Platforms without a POSIX
		// terminal (Windows) have no modes to change; the calls are
		// resolved at compile time there.
		if pinentry.TerminalHandoverSupported {
			pinentry.ReapplyTerminalModes()
			// pinentry-curses draws its dialog the moment it starts, while
			// the renderer may still own the screen, and it does not redraw
			// on SIGWINCH. The handover can therefore erase the first frame,
			// so leave a hint on the normal screen. Masked-input feedback
			// while typing still comes from pinentry itself.
			fmt.Fprintln(os.Stdout, "\nGPG is asking for your passphrase on this terminal (pinentry). Type it and press Enter; input is hidden.")
			if ctrlL {
				// Ncurses pinentry flavors repaint their full dialog on
				// Ctrl-L, recovering the frame the handover may have erased.
				// If the injection fails (e.g. TIOCSTI disabled on Linux),
				// the hint above remains as the fallback.
				pinentry.InjectCtrlLRedraw()
			}
		}
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
	return nil
}

// handlePinentryNotification dismisses the pinentry dialog once the
// prompt is resolved, covering the case where another subscriber (or a
// cancelled GPG command) resolved it first.
func (m *UI) handlePinentryNotification(_ pinentry.Notification) {
	if m.dialog.ContainsDialog(dialog.PinentryID) {
		m.dialog.CloseDialog(dialog.PinentryID)
	}
}

// handlePinentryPromptError surfaces a failed credential attempt as a
// caution-level status notification the moment GPG rejects a passphrase
// or PIN, instead of waiting for (or printing inside) the retry dialog.
func (m *UI) handlePinentryPromptError(pe pinentry.PromptError) tea.Cmd {
	return util.ReportWarn(pe.Error)
}
