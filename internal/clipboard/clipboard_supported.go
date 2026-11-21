//go:build (darwin || linux || windows || freebsd || openbsd || netbsd) && !ios && !android

package clipboard

import (
	"context"
	"sync"
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
