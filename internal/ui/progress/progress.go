// Package progress renders a fine-grained braille progress meter inspired by
// the Docker Compose v2 progress bar. Each cell encodes up to 8 dots, giving
// 8x sub-cell resolution per character.
package progress

import "math/rand/v2"

// glyphs are the nine fill stages of a single braille cell, adding one dot at
// a time bottom-up and left-before-right within each row.
var glyphs = [9]rune{
	'\u2800', // ⠀ 0/8
	'\u2840', // ⡀ 1/8
	'\u28c0', // ⣀ 2/8
	'\u28c4', // ⣄ 3/8
	'\u28e4', // ⣤ 4/8
	'\u28e6', // ⣦ 5/8
	'\u28f6', // ⣶ 6/8
	'\u28f7', // ⣷ 7/8
	'\u28ff', // ⣿ 8/8
}

// Render returns a width-cell braille meter representing percent in [0, 1].
// Values outside that range are clamped. A non-positive width yields the
// empty string.
func Render(width int, percent float64) string {
	if width <= 0 {
		return ""
	}
	switch {
	case percent < 0:
		percent = 0
	case percent > 1:
		percent = 1
	}

	totalDots := width * 8
	filled := int(percent*float64(totalDots) + 0.5)

	out := make([]rune, width)
	for i := range width {
		n := 0
		switch {
		case filled >= 8:
			n = 8
			filled -= 8
		case filled > 0:
			n = filled
			filled = 0
		}
		out[i] = glyphs[n]
	}
	return string(out)
}

// RenderSeeded is like [Render], but the seed deterministically permutes the
// cell fill order so cells do not necessarily fill left to right. Within each
// cell, dots still fill bottom-up and left-before-right.
func RenderSeeded(width int, percent float64, seed int) string {
	if width <= 0 {
		return ""
	}
	switch {
	case percent < 0:
		percent = 0
	case percent > 1:
		percent = 1
	}

	order := make([]int, width)
	for i := range order {
		order[i] = i
	}
	s := uint64(seed)
	r := rand.New(rand.NewPCG(s, s^0x9e3779b97f4a7c15))
	r.Shuffle(width, func(i, j int) {
		order[i], order[j] = order[j], order[i]
	})

	totalDots := width * 8
	filled := int(percent*float64(totalDots) + 0.5)

	out := make([]rune, width)
	for i := range out {
		out[i] = glyphs[0]
	}
	for _, idx := range order {
		n := 0
		switch {
		case filled >= 8:
			n = 8
			filled -= 8
		case filled > 0:
			n = filled
			filled = 0
		}
		out[idx] = glyphs[n]
	}
	return string(out)
}
