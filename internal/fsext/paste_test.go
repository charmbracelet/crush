package fsext

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPasteStringToPaths(t *testing.T) {
	t.Run("Windows", func(t *testing.T) {
		tests := []struct {
			name     string
			input    string
			expected []string
		}{
			{
				name:     "single path",
				input:    `"C:\path\my-screenshot-one.png"`,
				expected: []string{`C:\path\my-screenshot-one.png`},
			},
			{
				name:     "multiple paths no spaces",
				input:    `"C:\path\my-screenshot-one.png" "C:\path\my-screenshot-two.png" "C:\path\my-screenshot-three.png"`,
				expected: []string{`C:\path\my-screenshot-one.png`, `C:\path\my-screenshot-two.png`, `C:\path\my-screenshot-three.png`},
			},
			{
				name:     "sigle with spaces",
				input:    `"C:\path\my screenshot one.png"`,
				expected: []string{`C:\path\my screenshot one.png`},
			},
			{
				name:     "multiple paths with spaces",
				input:    `"C:\path\my screenshot one.png" "C:\path\my screenshot two.png" "C:\path\my screenshot three.png"`,
				expected: []string{`C:\path\my screenshot one.png`, `C:\path\my screenshot two.png`, `C:\path\my screenshot three.png`},
			},
			{
				name:     "empty string",
				input:    "",
				expected: nil,
			},
			{
				name:     "unclosed quotes",
				input:    `"C:\path\file.png`,
				expected: nil,
			},
			{
				name:     "text outside quotes",
				input:    `"C:\path\file.png" some random text "C:\path\file2.png"`,
				expected: []string{`C:\path\file.png`, `C:\path\file2.png`},
			},
			{
				name:     "multiple spaces between paths",
				input:    `"C:\path\file1.png"    "C:\path\file2.png"`,
				expected: []string{`C:\path\file1.png`, `C:\path\file2.png`},
			},
			{
				name:     "just whitespace",
				input:    "   ",
				expected: nil,
			},
			{
				name:     "consecutive quoted sections",
				input:    `"C:\path1""C:\path2"`,
				expected: []string{`C:\path1`, `C:\path2`},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := windowsPasteStringToPaths(tt.input)
				require.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("Unix", func(t *testing.T) {
		tests := []struct {
			name     string
			input    string
			expected []string
		}{
			{
				name:     "single path",
				input:    `/path/my-screenshot.png`,
				expected: []string{"/path/my-screenshot.png"},
			},
			{
				name:     "multiple paths no spaces",
				input:    `/path/screenshot-one.png /path/screenshot-two.png /path/screenshot-three.png`,
				expected: []string{"/path/screenshot-one.png", "/path/screenshot-two.png", "/path/screenshot-three.png"},
			},
			{
				name:     "sigle with spaces",
				input:    `/path/my\ screenshot\ one.png`,
				expected: []string{"/path/my screenshot one.png"},
			},
			{
				name:     "multiple paths with spaces",
				input:    `/path/my\ screenshot\ one.png /path/my\ screenshot\ two.png /path/my\ screenshot\ three.png`,
				expected: []string{"/path/my screenshot one.png", "/path/my screenshot two.png", "/path/my screenshot three.png"},
			},
			{
				name:     "empty string",
				input:    "",
				expected: nil,
			},
			{
				name:     "double backslash escapes",
				input:    `/path/my\\file.png`,
				expected: []string{"/path/my\\file.png"},
			},
			{
				name:     "trailing backslash",
				input:    `/path/file\`,
				expected: []string{`/path/file\`},
			},
			{
				name:     "multiple consecutive escaped spaces",
				input:    `/path/file\ \ with\ \ many\ \ spaces.png`,
				expected: []string{"/path/file  with  many  spaces.png"},
			},
			{
				name:     "multiple unescaped spaces",
				input:    `/path/file1.png   /path/file2.png`,
				expected: []string{"/path/file1.png", "/path/file2.png"},
			},
			{
				name:     "just whitespace",
				input:    "   ",
				expected: nil,
			},
			{
				name:     "tab characters",
				input:    "/path/file1.png\t/path/file2.png",
				expected: []string{"/path/file1.png\t/path/file2.png"},
			},
			{
				name:     "newlines in input",
				input:    "/path/file1.png\n/path/file2.png",
				expected: []string{"/path/file1.png\n/path/file2.png"},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := unixPasteStringToPaths(tt.input)
				require.Equal(t, tt.expected, result)
			})
		}
	})
}
