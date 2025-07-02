//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !aix && !zos && !windows
// +build !darwin,!dragonfly,!freebsd,!linux,!netbsd,!openbsd,!solaris,!aix,!zos,!windows

package uv

func (*Terminal) makeRaw() error {
	return ErrPlatformNotSupported
}

func (*Terminal) getSize() (int, int, error) {
	return 0, 0, ErrPlatformNotSupported
}

func (t *Terminal) optimizeMovements() {}

func (*Terminal) enableWindowsMouse() error  { return ErrPlatformNotSupported }
func (*Terminal) disableWindowsMouse() error { return ErrPlatformNotSupported }
