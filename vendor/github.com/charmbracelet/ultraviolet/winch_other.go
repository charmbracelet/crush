//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !aix && !zos
// +build !darwin,!dragonfly,!freebsd,!linux,!netbsd,!openbsd,!solaris,!aix,!zos

package uv

import (
	"context"
	"fmt"

	"github.com/charmbracelet/x/term"
)

func (*WinChReceiver) receiveEvents(context.Context, term.File, chan<- Event) error {
	return fmt.Errorf("SIGWINCH not supported on this platform")
}
