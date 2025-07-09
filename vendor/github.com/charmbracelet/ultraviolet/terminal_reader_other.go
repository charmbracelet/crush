//go:build !windows
// +build !windows

package uv

import "context"

// ReceiveEvents reads input events from the terminal and sends them to the
// given event channel.
func (d *TerminalReader) ReceiveEvents(ctx context.Context, events chan<- Event) error {
	return d.receiveEvents(ctx, events)
}

// parseWin32InputKeyEvent parses a Win32 input key events. This function is
// only available on Windows.
func (p *SequenceParser) parseWin32InputKeyEvent(*win32InputState, uint16, uint16, rune, bool, uint32, uint16, Logger) Event {
	return nil
}
