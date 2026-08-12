//go:build windows
// +build windows

package cmd

import "os"

func addSignals(sigs []os.Signal) []os.Signal {
	return sigs
}
