//go:build !windows

package cmd

import (
	"errors"
	"syscall"
)

// isConnRefused reports whether err is a refused tcp connection.
func isConnRefused(err error) bool {
	return errors.Is(err, syscall.ECONNREFUSED)
}
