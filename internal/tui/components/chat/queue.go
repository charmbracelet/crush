package chat

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/tui/styles"
)

func queuePill(queue int, t *styles.Theme) string {
	if queue <= 0 {
		return ""
	}

	// Generate triangles based on queue size (max 10)
	triangleCount := queue
	if triangleCount > 10 {
		triangleCount = 10
	}
	triangleChar := "▶"
	triangleString := strings.Repeat(triangleChar, triangleCount)

	// Format the queue number as a complete string OUTSIDE the border
	queueNumber := fmt.Sprintf("%d", queue)

	// Format the "Queued" text
	queueText := "Queued"

	// Apply gradient to triangles
	styledTriangles := styles.ApplyForegroundGrad(triangleString, t.RedDark, t.Accent)

	// Build the content inside the border (only triangles + Queued text)
	borderContent := styledTriangles + " " + queueText

	// Create border style - only contains triangles and "Queued"
	bordered := t.S().Base.
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(t.BgOverlay).
		PaddingLeft(1).
		PaddingRight(1).
		Render(borderContent)

	// Build number line separately, completely outside lipgloss processing
	// Add padding to align with the bordered content (4 spaces external + 1 border + 1 padding)
	numberLine := strings.Repeat(" ", 6) + queueNumber

	// Combine: number on top, bordered content below
	// Using simple string concatenation to avoid any lipgloss processing of the number
	result := numberLine + "\n" + strings.Repeat(" ", 4) + bordered

	return result
}
