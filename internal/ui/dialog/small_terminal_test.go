package dialog

import (
	"image"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// drawFits draws d into a w×h screen and asserts the whole dialog frame
// landed on screen: the first and last drawn rows are the complete top and
// bottom borders (a dialog taller or wider than the area is clipped by
// DrawCenterCursor, losing its bottom or right edge), no line is wider than
// the area, and any cursor lies inside the area. It returns the screen
// lines.
func drawFits(t *testing.T, d interface {
	Draw(uv.Screen, uv.Rectangle) *tea.Cursor
}, w, h int,
) []string {
	t.Helper()
	scr := uv.NewScreenBuffer(w, h)
	cur := d.Draw(scr, image.Rect(0, 0, w, h))
	lines := strings.Split(scr.String(), "\n")
	require.LessOrEqual(t, len(lines), h)
	for i, l := range lines {
		require.LessOrEqualf(t, ansi.StringWidth(l), w, "line %d too wide: %q", i, l)
	}
	text := strings.Join(lines, "\n")
	var rows []string
	for _, l := range lines {
		if l = strings.TrimSpace(l); l != "" {
			rows = append(rows, l)
		}
	}
	require.NotEmpty(t, rows, "nothing drawn at %dx%d", w, h)
	top, bottom := rows[0], rows[len(rows)-1]
	require.Truef(t, strings.HasPrefix(top, "╭") && strings.HasSuffix(top, "╮"), "dialog top border clipped at %dx%d:\n%s", w, h, text)
	require.Truef(t, strings.HasPrefix(bottom, "╰") && strings.HasSuffix(bottom, "╯"), "dialog bottom border clipped at %dx%d:\n%s", w, h, text)
	if cur != nil {
		require.Less(t, cur.Y, h, "cursor below the area")
		require.Less(t, cur.X, w, "cursor right of the area")
	}
	return lines
}
