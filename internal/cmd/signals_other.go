//go:build !windows

package cmd

import (
	"os"
	"syscall"
)

// terminationSignals lists the signals that mean "stop now, but tidily".
//
// Interrupt alone is not enough: closing a terminal window sends SIGHUP and
// service managers send SIGTERM, and both kill the process outright unless
// they are caught. Catching them lets the ordinary shutdown run, which is
// what records interrupted work instead of abandoning it mid-write.
func terminationSignals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP}
}
