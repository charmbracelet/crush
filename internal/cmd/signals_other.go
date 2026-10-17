//go:build !windows

package cmd

import (
	"os"
	"syscall"
)

// terminationSignals lists the signals that mean "stop now, but tidily":
// SIGHUP from a closed terminal and SIGTERM from a service manager.
func terminationSignals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP}
}
