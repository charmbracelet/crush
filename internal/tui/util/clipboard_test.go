package util

import (
	"bytes"
	"testing"
)

func TestEncodeUTF16LE(t *testing.T) {
	got, err := encodeUTF16LE("你好")
	if err != nil {
		t.Fatalf("encodeUTF16LE returned error: %v", err)
	}

	want := []byte{0x60, 0x4f, 0x7d, 0x59}
	if !bytes.Equal(got, want) {
		t.Fatalf("encodeUTF16LE = %v, want %v", got, want)
	}
}

func TestEncodeUTF16LEWithBOM(t *testing.T) {
	got, err := encodeUTF16LEWithBOM("你好")
	if err != nil {
		t.Fatalf("encodeUTF16LEWithBOM returned error: %v", err)
	}

	want := []byte{0xff, 0xfe, 0x60, 0x4f, 0x7d, 0x59}
	if !bytes.Equal(got, want) {
		t.Fatalf("encodeUTF16LEWithBOM = %v, want %v", got, want)
	}
}
