//go:build windows

package cmd

// detachStderr is a no-op: Windows has no fork step for children to print in.
func detachStderr() (restore func(), err error) {
	return func() {}, nil
}
