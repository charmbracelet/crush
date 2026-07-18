//go:build !crush_omit_native && (darwin || linux || windows || freebsd || openbsd || netbsd) && !ios && !android

package clipboard

import "golang.design/x/clipboard"

func initClipboard() error {
	return clipboard.Init()
}

func writeText(text string) {
	clipboard.Write(clipboard.FmtText, []byte(text))
}

func read(f Format) ([]byte, error) {
	var format clipboard.Format
	switch f {
	case FormatText:
		format = clipboard.FmtText
	case FormatImage:
		format = clipboard.FmtImage
	default:
		return nil, ErrEmpty
	}
	data := clipboard.Read(format)
	if data == nil {
		return nil, ErrEmpty
	}
	return data, nil
}
