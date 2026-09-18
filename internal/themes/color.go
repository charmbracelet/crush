package themes

import (
	"image/color"
	"strconv"
	"strings"

	"github.com/lucasb-eyer/go-colorful"
)

// parseHex parses a "#RRGGBB" string into a color, returning nil on failure.
func parseHex(s string) color.Color {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return nil
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return nil
	}
	return color.RGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff}
}

func firstNonNil(cs ...color.Color) color.Color {
	for _, c := range cs {
		if c != nil {
			return c
		}
	}
	return color.RGBA{}
}

// blend mixes t of b into a in RGB space, returning a non-nil color even when
// a or b is nil.
func blend(a, b color.Color, t float64) color.Color {
	ca, _ := colorful.MakeColor(firstNonNil(a))
	cb, _ := colorful.MakeColor(firstNonNil(b))
	return ca.BlendRgb(cb, t)
}

// contrastText returns black or white, whichever reads better on bg, using
// relative luminance as the decision metric.
func contrastText(bg color.Color) color.Color {
	r, g, b, _ := firstNonNil(bg).RGBA()
	lum := (0.299*float64(r>>8) + 0.587*float64(g>>8) + 0.114*float64(b>>8)) / 255
	if lum > 0.5 {
		return color.RGBA{R: 0, G: 0, B: 0, A: 0xff}
	}
	return color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
}
