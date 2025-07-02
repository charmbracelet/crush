//go:build !appengine && !js && go1.21
// +build !appengine,!js,go1.21

package util

import (
	"unsafe"
)

// BytesToReadOnlyString returns a string converted from given bytes.
func BytesToReadOnlyString(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}

// StringToReadOnlyBytes returns bytes converted from given string.
func StringToReadOnlyBytes(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}
