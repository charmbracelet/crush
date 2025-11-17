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
	triangles := styles.ForegroundGrad("▶▶▶▶▶▶▶▶▶", false, t.RedDark, t.Accent)
	if queue < 10 {
		triangles = triangles[:queue]
	}

	allTriangles := strings.Join(triangles, "")

	return t.S().Base.
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(t.BgOverlay).
		PaddingLeft(1).
		PaddingRight(1).
		Render(fmt.Sprintf("%s %d Queued", allTriangles, queue))
}
