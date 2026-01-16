//go:build linux && 386

package editor

func readClipboard(clipboardFormat) ([]byte, error) {
	return nil, errClipboardPlatformUnsupported
}
