// Package progress renders a fine-grained braille progress meter inspired by
// the Docker Compose v2 progress bar. Each cell encodes up to 8 dots, giving
// 8x sub-cell resolution per character.
package progress

import (
	"image/color"
	"math/rand/v2"

	uv "github.com/charmbracelet/ultraviolet"
)

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

// Render returns a width-cell braille meter representing percent in [0, 1],
// styled with the given foreground and background colors. Either color may
// be nil to leave that channel at the terminal default. Values outside the
// percent range are clamped. A non-positive width yields the empty string.
func Render(width int, percent float64, fg, bg color.Color) string {
	if width <= 0 {
		return ""
	}
	return style(fg, bg, fill(width, percent))
}

// RenderSeeded is like [Render], but the seed deterministically permutes the
// cell fill order so cells do not necessarily fill left to right. Within each
// cell, dots still fill bottom-up and left-before-right.
func RenderSeeded(width int, percent float64, fg, bg color.Color, seed int) string {
	if width <= 0 {
		return ""
	}
	return style(fg, bg, shuffle(fill(width, percent), seed))
}

// fill returns width braille cells filled left-to-right to represent percent.
func fill(width int, percent float64) []rune {
	switch {
	case percent < 0:
		percent = 0
	case percent > 1:
		percent = 1
	}
	filled := int(percent*float64(width*8) + 0.5)
	out := make([]rune, width)
	for i := range out {
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
	return out
}

// shuffle reorders cells according to a deterministic permutation derived
// from seed, so fully-filled cells scatter across the bar instead of packing
// to the left.
func shuffle(cells []rune, seed int) []rune {
	s := uint64(seed)
	r := rand.New(rand.NewPCG(s, s^0x9e3779b97f4a7c15))
	order := make([]int, len(cells))
	for i := range order {
		order[i] = i
	}
	r.Shuffle(len(order), func(i, j int) {
		order[i], order[j] = order[j], order[i]
	})
	out := make([]rune, len(cells))
	for i, idx := range order {
		out[idx] = cells[i]
	}
	return out
}

// style wraps cells in the ANSI sequences for the given foreground and
// background colors. A nil color leaves that channel unset.
func style(fg, bg color.Color, cells []rune) string {
	s := uv.Style{Fg: fg, Bg: bg}
	if s.IsZero() {
		return string(cells)
	}
	return s.Styled(string(cells))
}
