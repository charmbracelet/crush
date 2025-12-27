package ansiext

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Escape replaces control characters with their Unicode Control Picture
// representations to ensure they are displayed correctly in the UI.
func Escape(content string) string {
	var sb strings.Builder
	sb.Grow(len(content))
	for _, r := range content {
		switch {
		case r >= 0 && r <= 0x1f: // Control characters 0x00-0x1F
			sb.WriteRune('\u2400' + r)
		case r == ansi.DEL:
			sb.WriteRune('\u2421')
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// isLetter returns true if the rune is an ASCII letter (A-Z or a-z).
func isLetter(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')
}

// EscapePreservingANSI escapes control characters while preserving ANSI escape
// sequences for color/style rendering. This is useful for terminal output that
// contains intentional ANSI color codes (e.g., bash command output).
func EscapePreservingANSI(content string) string {
	var sb strings.Builder
	sb.Grow(len(content))
	runes := []rune(content)
	i := 0
	for i < len(runes) {
		r := runes[i]
		// Detect ANSI escape sequence start: ESC followed by '['
		if r == '\x1b' && i+1 < len(runes) && runes[i+1] == '[' {
			// Find the end of the ANSI sequence (terminated by a letter)
			j := i + 2
			foundTerminator := false
			for j < len(runes) {
				c := runes[j]
				if isLetter(c) {
					// Found sequence terminator, preserve the entire sequence
					for k := i; k <= j; k++ {
						sb.WriteRune(runes[k])
					}
					i = j + 1
					foundTerminator = true
					break
				}
				// Valid ANSI sequence characters: digits, semicolons, and question mark
				if (c >= '0' && c <= '9') || c == ';' || c == '?' {
					j++
					continue
				}
				// Invalid character in sequence, stop searching
				break
			}
			if !foundTerminator {
				// Incomplete or invalid sequence, escape the ESC character only
				// and let the rest be processed normally
				sb.WriteRune('\u241B')
				i++
			}
			continue
		}
		// Normal control character handling
		switch {
		case r >= 0 && r <= 0x1f: // Control characters 0x00-0x1F
			sb.WriteRune('\u2400' + r)
		case r == ansi.DEL:
			sb.WriteRune('\u2421')
		default:
			sb.WriteRune(r)
		}
		i++
	}
	return sb.String()
}
