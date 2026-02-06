//go:build !linux || !386

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
