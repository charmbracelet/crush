//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !aix && !zos && !windows
// +build !darwin,!dragonfly,!freebsd,!linux,!netbsd,!openbsd,!solaris,!aix,!zos,!windows

package uv

import "os"

func openTTY() (*os.File, *os.File, error) {
	return nil, nil, ErrPlatformNotSupported
}

func suspend() error {
	return ErrPlatformNotSupported
}
