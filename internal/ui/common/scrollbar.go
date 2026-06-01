package common

import (
	"strings"

	"github.com/charmbracelet/crush/internal/ui/styles"
)

// Scrollbar renders a vertical scrollbar based on content and viewport size.
// Returns an empty string if content fits within viewport (no scrolling needed).
func Scrollbar(s *styles.Styles, height, contentSize, viewportSize, offset int) string {
	if height <= 0 || contentSize <= viewportSize {
		return ""
	}

	// Calculate thumb size (minimum 1 character).
	thumbSize := max(1, height*viewportSize/contentSize)

	// Calculate thumb position.
	maxOffset := contentSize - viewportSize
	if maxOffset <= 0 {
		return ""
	}

	// Calculate where the thumb starts.
	trackSpace := height - thumbSize
	thumbPos := 0
	if trackSpace > 0 && maxOffset > 0 {
		thumbPos = min(trackSpace, offset*trackSpace/maxOffset)
	}

	// Build the scrollbar.
	var sb strings.Builder
	for i := range height {
		if i > 0 {
			sb.WriteString("\n")
		}
		if i >= thumbPos && i < thumbPos+thumbSize {
			sb.WriteString(s.Dialog.ScrollbarThumb.Render(styles.ScrollbarThumb))
		} else {
			sb.WriteString(s.Dialog.ScrollbarTrack.Render(styles.ScrollbarTrack))
		}
	}

	return sb.String()
}
