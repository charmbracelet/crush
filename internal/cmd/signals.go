package cmd

import (
	"os"
	"syscall"
)

// terminationSignals are the signals that mean "stop now, but tidily":
// Ctrl+C, SIGTERM from a service manager (Go also delivers a closed Windows
// console as SIGTERM), and SIGHUP from a closed terminal. The server reads
// SIGHUP as a config reload instead; see awaitStop.
func terminationSignals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP}
}
