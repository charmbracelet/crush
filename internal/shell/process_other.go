//go:build !unix && !windows

package shell

import (
	"os/exec"
	"time"
)

// killTimeout is zero on non-Unix platforms because Go's os.Process.Kill
// is the only portable termination primitive; there is no SIGINT analogue
// to send first. Kept as a named constant so the call sites read the same
// on all platforms.
const killTimeout = 0 * time.Second

// prepareCmd is a no-op outside Unix. True process-tree kill on Windows
// requires JobObjects; that is out of scope here. The handler still
// returns control to the caller on cancel, but grandchildren may survive.
func prepareCmd(cmd *exec.Cmd) {}

// interruptCmd is implemented as a hard kill on non-Unix because
// os.Process.Signal(os.Interrupt) is not supported on Windows.
func interruptCmd(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}

// killCmd hard-kills the direct child. See prepareCmd for the
// grandchild-leak caveat on Windows.
func killCmd(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
