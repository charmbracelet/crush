//go:build windows

package cmd

import "os"

// terminationSignals lists the signals that mean "stop now, but tidily".
// Windows has neither SIGTERM nor SIGHUP, so Interrupt is all there is.
func terminationSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}
