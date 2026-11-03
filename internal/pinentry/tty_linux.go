//go:build linux

package pinentry

import "golang.org/x/sys/unix"

func getTermios(fd int) (*unix.Termios, error) {
	return unix.IoctlGetTermios(fd, unix.TCGETS)
}

func setTermios(fd int, tio *unix.Termios) error {
	return unix.IoctlSetTermios(fd, unix.TCSETS, tio)
}
