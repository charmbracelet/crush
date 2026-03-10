//go:build (linux || darwin) && !arm && !386 && !ios && !android

package model

import "github.com/aymanbagabas/go-nativeclipboard"

func readClipboard(f clipboardFormat) ([]byte, error) {
	switch f {
	case clipboardFormatText:
		return nativeclipboard.Text.Read()
	case clipboardFormatImage:
		return nativeclipboard.Image.Read()
	}
	return nil, errClipboardUnknownFormat
}

func readClipboardFileList() ([]string, error) {
	return nil, errClipboardUnknownFormat
}
