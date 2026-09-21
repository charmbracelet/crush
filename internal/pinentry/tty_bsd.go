//go:build darwin || freebsd || netbsd || openbsd

package pinentry

import "golang.org/x/sys/unix"

func getTermios(fd int) (*unix.Termios, error) {
	return unix.IoctlGetTermios(fd, unix.TIOCGETA)
}

func setTermios(fd int, tio *unix.Termios) error {
	return unix.IoctlSetTermios(fd, unix.TIOCSETA, tio)
}
