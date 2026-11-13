package dialog

import (
	"image"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// paintedTextPoint finds text independently of the component's hit geometry.
func paintedTextPoint(t *testing.T, screen uv.ScreenBuffer, text string) image.Point {
	t.Helper()
	for y, line := range strings.Split(ansi.Strip(screen.Render()), "\n") {
		if before, _, ok := strings.Cut(line, text); ok {
			return image.Pt(ansi.StringWidth(before), y)
		}
	}
	t.Fatalf("Text %q not visible in:\n%s", text, screen.Render())
	return image.Point{}
}

// drawScreen draws a dialog or inline editor into a fresh screen that ends
// where area does.
func drawScreen(d interface {
	Draw(uv.Screen, uv.Rectangle) *tea.Cursor
}, area image.Rectangle,
) uv.ScreenBuffer {
	scr := uv.NewScreenBuffer(max(1, area.Max.X), max(1, area.Max.Y))
	d.Draw(scr, area)
	return scr
}
