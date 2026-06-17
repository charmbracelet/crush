//go:build darwin && !ios

package model

import (
	"bytes"
	"image/png"
	"runtime"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"golang.org/x/image/tiff"
)

var (
	nsPasteboardClass objc.Class
	nsDataClass       objc.Class

	selGeneralPasteboard objc.SEL
	selDataForType       objc.SEL
	selBytes             objc.SEL
	selLength            objc.SEL

	nsPasteboardTypeTIFF objc.ID

	clipboardImageDarwinInitialized bool
	clipboardImageDarwinInitError   error
)

func initClipboardImageDarwin() {
	if clipboardImageDarwinInitialized {
		return
	}
	clipboardImageDarwinInitialized = true

	appkit, err := purego.Dlopen("/System/Library/Frameworks/AppKit.framework/AppKit", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		clipboardImageDarwinInitError = err
		return
	}

	nsPasteboardClass = objc.GetClass("NSPasteboard")
	nsDataClass = objc.GetClass("NSData")

	selGeneralPasteboard = objc.RegisterName("generalPasteboard")
	selDataForType = objc.RegisterName("dataForType:")
	selBytes = objc.RegisterName("bytes")
	selLength = objc.RegisterName("length")

	typeTIFFPtr, err := purego.Dlsym(appkit, "NSPasteboardTypeTIFF")
	if err != nil {
		clipboardImageDarwinInitError = err
		return
	}
	nsPasteboardTypeTIFF = objc.ID(*(*uintptr)(unsafe.Pointer(typeTIFFPtr)))
}

// readClipboardImageFallback attempts to read image data from the macOS
// clipboard in formats other than PNG (e.g., TIFF from WeChat screenshots).
// It returns the image encoded as PNG.
func readClipboardImageFallback() ([]byte, error) {
	initClipboardImageDarwin()
	if clipboardImageDarwinInitError != nil {
		return nil, clipboardImageDarwinInitError
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	pasteboard := objc.ID(nsPasteboardClass).Send(selGeneralPasteboard)
	if pasteboard == 0 {
		return nil, errClipboardImageUnavailable
	}

	data := pasteboard.Send(selDataForType, nsPasteboardTypeTIFF)
	if data == 0 {
		return nil, errClipboardImageUnavailable
	}

	length := objc.Send[uint64](data, selLength)
	if length == 0 {
		return nil, errClipboardImageUnavailable
	}

	bytesPtr := data.Send(selBytes)
	if bytesPtr == 0 {
		return nil, errClipboardImageUnavailable
	}

	buf := make([]byte, length)
	copyBytes(buf, uintptr(bytesPtr), int(length))

	img, err := tiff.Decode(bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}

	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// copyBytes copies n bytes from src to dst. It is defined locally to avoid
// depending on the implementation details of go-nativeclipboard.
func copyBytes(dst []byte, src uintptr, n int) {
	for i := range n {
		dst[i] = *(*byte)(unsafe.Pointer(src + uintptr(i)))
	}
}
