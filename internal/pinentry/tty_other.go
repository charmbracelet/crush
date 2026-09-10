//go:build !darwin && !linux && !freebsd && !netbsd && !openbsd

package pinentry

import "errors"

// SetTerminalRawNoEcho is unsupported on this platform.
func SetTerminalRawNoEcho() error {
	return errors.New("terminal mode configuration is not supported on this platform")
}
