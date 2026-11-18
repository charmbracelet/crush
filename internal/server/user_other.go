//go:build !windows

package server

import (
	"os"
	"strconv"
)

// currentUserID returns the uid of the current process, the same string
// [os/user.Current] reports as Uid. Unlike user.Current it cannot fail,
// whereas the pure-Go user.Current of cgo-less Linux builds only knows
// the users in /etc/passwd and otherwise needs $USER and $HOME.
func currentUserID() string {
	return strconv.Itoa(os.Getuid())
}
