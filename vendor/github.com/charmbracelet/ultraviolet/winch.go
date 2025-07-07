package uv

import (
	"context"
	"fmt"

	"github.com/charmbracelet/x/term"
)

// WinChReceiver listens for window size changes using (SIGWINCH) and sends the
// new size to the event channel.
// This is a Unix-specific implementation and should be used on Unix-like
// systems and won't work on Windows.
type WinChReceiver struct{ term.File }

// Start starts the receiver.
func (l *WinChReceiver) Start() error {
	if l.File == nil {
		return fmt.Errorf("no file set")
	}
	_, _, err := term.GetSize(l.File.Fd())
	return err
}

// ReceiveEvents listens for window size changes and sends the new size to the
// event channel. It stops when the context is done or an error occurs.
func (l *WinChReceiver) ReceiveEvents(ctx context.Context, evch chan<- Event) error {
	return l.receiveEvents(ctx, l.File, evch)
}
