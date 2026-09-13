// Package clipboard provides cross-platform clipboard access behind build-tag
// guards so that unsupported platforms (e.g., Android, iOS) compile without
// requiring CGO or platform-specific dependencies.
package clipboard

import "errors"

// Format identifies the kind of data to read from the clipboard.
type Format int

const (
	// FormatText is plain UTF-8 text.
	FormatText Format = iota
	// FormatImage is binary image data (PNG).
	FormatImage
)

var (
	// ErrUnsupported is returned when clipboard access is not available on
	// the current platform.
	ErrUnsupported = errors.New("clipboard operations are not supported on this platform")
	// ErrEmpty is returned when the clipboard holds no data for the
	// requested format.
	ErrEmpty = errors.New("clipboard is empty or holds an unsupported format")
	// ErrWriteFailed is returned when the platform has a clipboard but the
	// written text is not there afterwards.
	ErrWriteFailed = errors.New("clipboard write did not land")
)

// Init initializes the clipboard subsystem. On unsupported platforms it
// returns ErrUnsupported but is otherwise safe to call.
func Init() error {
	return initClipboard()
}

// WriteText writes plain text to the system clipboard and reads it back to
// confirm the write landed. It returns ErrUnsupported on platforms without a
// clipboard or when Init did not succeed, and ErrWriteFailed when the
// clipboard does not hold the text afterwards.
func WriteText(text string) error {
	return writeText(text)
}

// Read returns the clipboard contents for the given format. It returns
// ErrEmpty when no matching data is present and ErrUnsupported on platforms
// without clipboard support.
func Read(f Format) ([]byte, error) {
	return read(f)
}
