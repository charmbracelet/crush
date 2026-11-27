//go:build darwin || linux || freebsd || netbsd || openbsd

package pinentry

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/charmbracelet/x/ansi"
	"golang.org/x/sys/unix"
)

const handoverHint = "GPG is asking for your passphrase on this terminal (pinentry). Type it and press Enter; input is hidden."

// BeginHandover prepares a just-released terminal for a pinentry dialog.
// The hint lives on a scratch alt screen so it never reaches scrollback.
func BeginHandover(ctrlLRedraw bool) {
	if err := SetTerminalRawNoEcho(); err != nil {
		slog.Debug("Failed to configure terminal modes for pinentry", "error", err)
	}
	fmt.Fprint(os.Stdout, ansi.SetModeAltScreenSaveCursor+ansi.EraseEntireScreen+ansi.CursorHomePosition+handoverHint+"\r\n")
	if ctrlLRedraw {
		injectCtrlLRedraw()
	}
}

// EndHandover discards the scratch screen BeginHandover drew on.
func EndHandover() {
	fmt.Fprint(os.Stdout, ansi.ResetModeAltScreenSaveCursor)
}

// SetTerminalRawNoEcho puts the controlling terminal into non-canonical,
// no-echo mode, approximating the termios a pinentry dialog configures
// when it starts.
//
// This is needed because pinentry sets its terminal modes the moment it
// starts, while Crush still owns the terminal; the terminal handover then
// restores the pre-Crush (canonical, echo) state, clobbering pinentry's
// modes on the shared tty. Without this call the dialog's input would be
// line-buffered and echoed in cleartext, and an injected Ctrl-L would
// never reach its input loop. Canonical mode is left off but all other
// flags (including ICRNL, so Enter still produces a newline) are
// preserved.
func SetTerminalRawNoEcho() error {
	// O_RDWR is required: macOS denies some ioctls on write-only handles.
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("failed to open /dev/tty: %w", err)
	}
	defer tty.Close() //nolint:errcheck

	tio, err := getTermios(int(tty.Fd()))
	if err != nil {
		return fmt.Errorf("failed to read terminal attributes: %w", err)
	}
	tio.Lflag &^= unix.ICANON | unix.ECHO
	if err := setTermios(int(tty.Fd()), tio); err != nil {
		return fmt.Errorf("failed to set terminal attributes: %w", err)
	}
	return nil
}
