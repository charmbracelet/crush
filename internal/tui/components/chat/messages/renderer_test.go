package messages

import (
	"testing"
)

func TestEscapeContent(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "nothing to escape",
			input:    "Hello, World!",
			expected: "Hello, World!",
		},
		{
			name:     "escape csi sequences",
			input:    "\x1b[31mRed Text\x1b[0m",
			expected: "\\x1b[31mRed Text\\x1b[0m",
		},
		{
			name:     "escape control characters",
			input:    "Hello\x00World\x7f!",
			expected: "Hello\\x00World\\x7f!",
		},
		{
			name:     "escape csi sequences with control characters",
			input:    "\x1b[31mHello\x00World\x7f!\x1b[0m",
			expected: "\\x1b[31mHello\\x00World\\x7f!\\x1b[0m",
		},
		{
			name:     "just unicode",
			input:    "こんにちは", // "Hello" in Japanese
			expected: "こんにちは",
		},
		{
			name:     "unicode with csi sequences and control characters",
			input:    "\x1b[31mこんにちは\x00World\x7f!\x1b[0m",
			expected: "\\x1b[31mこんにちは\\x00World\\x7f!\\x1b[0m",
		},
	}
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			result := escapeContent(nil, c.input)
			if result != c.expected {
				t.Errorf("case %d, expected %q, got %q", i+1, c.expected, result)
			}
		})
	}
}
