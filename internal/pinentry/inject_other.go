//go:build !darwin && !linux && !freebsd && !netbsd && !openbsd

package pinentry

import "errors"

// InjectCtrlL is unsupported on this platform.
func InjectCtrlL() error {
	return errors.New("Ctrl-L injection is not supported on this platform")
}
