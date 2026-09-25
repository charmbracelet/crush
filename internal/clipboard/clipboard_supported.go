//go:build (darwin || linux || windows || freebsd || openbsd || netbsd) && !ios && !android

package clipboard

import (
	"context"
	"sync"

	"golang.design/x/clipboard"
)

// initOnce single-flights initialization: golang.design's Init starts the
// platform watcher (a pasteboard poll on macOS, a message listener on
// Windows), and paying that on every write added a visible delay to every
// copy. The exported Init goes through initClipboard, so repeated calls
// return the cached result without touching the watcher again.
var initOnce = sync.OnceValue(func() error {
	return clipboard.Init()
})

// ready reports whether the native clipboard is usable.
func ready() bool {
	return initOnce() == nil
}

func initClipboard() error {
	return initOnce()
}

func writeText(text string) error {
	if !ready() {
		return ErrUnsupported
	}
	// The write's own error is the failure signal. A read-back check here
	// cost a full clipboard round-trip per copy — a synchronous
	// NSPasteboard read on macOS, and an open-for-read that blocks while
	// another app holds the clipboard on Windows — which made every copy
	// feel delayed by hundreds of milliseconds.
	if _, err := clipboard.Write(context.Background(), clipboard.FmtText, []byte(text)); err != nil {
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
