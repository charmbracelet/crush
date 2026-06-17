//go:build !(darwin && !ios)

package model

// readClipboardImageFallback is a no-op on non-macOS platforms. The
// go-nativeclipboard library already handles the common image formats on
// Linux and Windows.
func readClipboardImageFallback() ([]byte, error) {
	return nil, errClipboardImageUnavailable
}
