//go:build darwin || linux || freebsd || netbsd || openbsd

package pinentry

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// ctrlL is the byte that makes ncurses-based pinentry dialogs redraw
// their UI.
const ctrlL = 0x0C

// InjectCtrlL writes a Ctrl-L byte into the controlling terminal's input
// queue via TIOCSTI, causing a curses-based pinentry dialog to redraw.
// This is needed because the terminal handover can erase the dialog frame
// that pinentry drew while Crush still owned the screen, and pinentry
// does not redraw on SIGWINCH.
//
// It returns an error when TIOCSTI is unavailable or disabled (some Linux
// kernels boot with dev.tty.legacy_tiocsti=0); callers should treat that
// as a best-effort failure and keep any fallback hint visible.
func InjectCtrlL() error {
	// O_RDWR is required: macOS denies TIOCSTI on write-only tty handles.
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("failed to open /dev/tty: %w", err)
	}
	defer tty.Close() //nolint:errcheck

	// TIOCSTI takes a pointer to a byte; IoctlSetPointerInt passes a
	// pointer to an int whose first byte the kernel reads (all supported
	// platforms are little-endian), so the byte is queued as if typed on
	// the terminal.
	if err := unix.IoctlSetPointerInt(int(tty.Fd()), unix.TIOCSTI, ctrlL); err != nil {
		return fmt.Errorf("TIOCSTI ioctl failed: %w", err)
	}
	return nil
}
