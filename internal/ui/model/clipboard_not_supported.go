//go:build !((darwin || linux || windows || freebsd || openbsd || netbsd) && !ios && !android)

package model

func readClipboard(clipboardFormat) ([]byte, error) {
	return nil, errClipboardPlatformUnsupported
}
