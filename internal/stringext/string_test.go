package stringext

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsValidBase64(t *testing.T) {
	t.Parallel()

	// Real PNG header encoded in standard base64.
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	pngBase64 := base64.StdEncoding.EncodeToString(pngHeader)

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{name: "empty string", input: "", expected: false},
		{name: "valid no padding", input: "SGVsbG8gV29ybGQh", expected: true},
		{name: "valid with padding", input: "YQ==", expected: true},
		{name: "non-ASCII bytes", input: "abc\x80def", expected: false},
		{name: "ASCII but not base64", input: "hello world!!!", expected: false},
		{name: "raw encoding no padding", input: "YQ", expected: false},
		{name: "trailing whitespace", input: "YQ==\n", expected: false},
		{name: "valid PNG header base64", input: pngBase64, expected: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, IsValidBase64(tt.input))
		})
	}
}
