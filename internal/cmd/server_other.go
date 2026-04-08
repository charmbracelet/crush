//go:build !windows
// +build !windows

package cmd

import (
	"os"
	"syscall"
)

func addSignals(sigs []os.Signal) []os.Signal {
	return append(sigs, syscall.SIGTERM)
}
