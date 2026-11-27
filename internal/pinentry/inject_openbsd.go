//go:build openbsd

package pinentry

import "errors"

// injectCtrlLRedraw is a no-op on OpenBSD, which removed the TIOCSTI
// ioctl; the hint printed on the terminal remains as the fallback.
func injectCtrlLRedraw() {}

// InjectCtrlL always fails on OpenBSD, which has no TIOCSTI.
func InjectCtrlL() error {
	return errors.New("TIOCSTI is not available on OpenBSD")
}
