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
