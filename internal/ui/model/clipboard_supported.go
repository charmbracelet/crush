//go:build (darwin || linux || windows || freebsd || openbsd || netbsd) && !ios && !android

package model

import "golang.design/x/clipboard"

func readClipboard(f clipboardFormat) ([]byte, error) {
	switch f {
	case clipboardFormatText:
		data := clipboard.Read(clipboard.FmtText)
		if data == nil {
			return nil, errClipboardUnknownFormat
		}
		return data, nil
	case clipboardFormatImage:
		data := clipboard.Read(clipboard.FmtImage)
		if data == nil {
			return nil, errClipboardUnknownFormat
		}
		return data, nil
	}
	return nil, errClipboardUnknownFormat
}
