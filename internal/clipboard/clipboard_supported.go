//go:build (darwin || linux || windows || freebsd || openbsd || netbsd) && !ios && !android

package clipboard

import (
	"bytes"

	"golang.design/x/clipboard"
)

// ready reports whether the native clipboard is usable. Touching the clipboard
// after a failed initialization may panic, and golang.design's Init is
// idempotent and cheap once it has run, so every entry point asks it again
// rather than tracking initialization state of our own.
func ready() bool {
	return clipboard.Init() == nil
}

func initClipboard() error {
	return clipboard.Init()
}

func writeText(text string) error {
	if !ready() {
		return ErrUnsupported
	}
	// A nil channel means the backend never took the clipboard; reading back
	// catches the rest, where the write is accepted but the text is not served
	// afterwards. Neither check subsumes the other: a failed write leaves an
	// earlier identical copy in place, which reads back as a success.
	if clipboard.Write(clipboard.FmtText, []byte(text)) == nil {
		return ErrWriteFailed
	}
	if !bytes.Equal(clipboard.Read(clipboard.FmtText), []byte(text)) {
		return ErrWriteFailed
	}
	return nil
}

func read(f Format) ([]byte, error) {
	if !ready() {
		return nil, ErrUnsupported
	}
	var format clipboard.Format
	switch f {
	case FormatText:
		format = clipboard.FmtText
	case FormatImage:
		format = clipboard.FmtImage
	default:
		return nil, ErrEmpty
	}
	data := clipboard.Read(format)
	if data == nil {
		return nil, ErrEmpty
	}
	return data, nil
}
