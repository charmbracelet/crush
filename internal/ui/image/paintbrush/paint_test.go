package paintbrush

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPaintCollectsEveryCell paints a gradient, which has no glyph ties, so
// every paint must match. Before Paint waited for its results, a paint could
// miss cells, and -race reported the unsynchronized reads.
func TestPaintCollectsEveryCell(t *testing.T) {
	t.Parallel()

	img := image.NewRGBA(image.Rect(0, 0, 64, 32))
	for y := range 32 {
		for x := range 64 {
			img.Set(x, y, color.RGBA{R: uint8(x * 4), G: uint8(y * 8), B: 128, A: 255})
		}
	}

	paint := func() string {
		c := New()
		c.SetImage(img)
		c.SetWidth(16)
		c.SetHeight(8)
		c.Paint()
		require.Equal(t, float32(1), c.GetProgress())
		return c.GetResult()
	}

	want := paint()
	require.Len(t, strings.Split(strings.TrimRight(want, "\n"), "\n"), 8)
	for range 5 {
		require.Equal(t, want, paint())
	}
}
