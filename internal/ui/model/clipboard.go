package model

import "errors"

type clipboardFormat int

const (
	clipboardFormatText clipboardFormat = iota
	clipboardFormatImage
)

var (
	errClipboardPlatformUnsupported = errors.New("clipboard operations are not supported on this platform")
	errClipboardUnknownFormat       = errors.New("unknown clipboard format")
	errClipboardImageUnavailable    = errors.New("clipboard does not contain a supported image format")
)
