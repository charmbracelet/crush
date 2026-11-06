//go:build (darwin || linux || windows || freebsd || openbsd || netbsd) && !ios && !android

package clipboard

import (
	"bytes"
	"context"
	"time"

	"golang.design/x/clipboard"
)

// A clipboard is shared with every other program on the machine, and a write
// is not one atomic step: the backend clears the clipboard and then sets the
// new contents. Another writer landing in that window makes a perfectly good
// write look lost, so try a few times before believing it.
const (
	writeAttempts = 3
	retryDelay    = 5 * time.Millisecond
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
	var err error
	for attempt := range writeAttempts {
		if attempt > 0 {
			time.Sleep(retryDelay)
		}
		if err = attemptWriteText(text); err == nil {
			return nil
		}
	}
	return err
}

func attemptWriteText(text string) error {
	// A write error means the backend never took the clipboard; reading back
	// catches the rest, where the write is accepted but the text is not served
	// afterwards. Neither check subsumes the other: a failed write leaves an
	// earlier identical copy in place, which reads back as a success.
	if _, err := clipboard.Write(context.Background(), clipboard.FmtText, []byte(text)); err != nil {
		return ErrWriteFailed
	}
	// Writing nothing empties the clipboard, so there is nothing to read back.
	if text == "" {
		return nil
	}
	data, err := clipboard.Read(context.Background(), clipboard.FmtText)
	if err != nil || !bytes.Equal(data, []byte(text)) {
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
	data, err := clipboard.Read(context.Background(), format)
	if err != nil || data == nil {
		return nil, ErrEmpty
	}
	return data, nil
}
