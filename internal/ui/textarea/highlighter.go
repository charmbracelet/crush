package textarea

import (
	"charm.land/lipgloss/v2"
	rw "github.com/mattn/go-runewidth"
)

// LineHighlighter is an optional hook that applies per-token styling to
// rendered lines. It is consulted once per logical line per render and
// returns style ranges over the line's runes: range Start (inclusive) and
// End (exclusive) are rune offsets within the line, and Style is composed
// with the line's base style by the renderer.
//
// The hook runs after wrapping: the textarea asks for ranges on the raw
// logical line, then intersects those ranges with each wrapped segment at
// render time. Implementations must therefore return ranges in rune offsets
// of the logical line they are given — never byte offsets, and never ranges
// derived from a different string.
//
// The cell under the virtual cursor is never styled by ranges: the renderer
// splits cursor-line content into pre- and post-cursor segments and renders
// the cursor cell itself separately.
//
// Keeping this interface range-based (rather than bubbline's
// func(string) string ANSI rewriting) means implementations never have to
// parse or preserve escape sequences, and the textarea's wrap/cursor math
// always operates on raw runes.
type LineHighlighter interface {
	// Highlight returns the style ranges for the given logical line.
	// lineIdx is the zero-based logical line number; line is the raw rune
	// content (no trailing newline). Implementations should return nil for
	// lines with nothing to style.
	Highlight(lineIdx int, line []rune) []lipgloss.Range
}

// SetHighlighter installs an optional line highlighter. Pass nil to disable
// highlighting. The highlighter is consulted at render time only; it does
// not affect editing, wrapping, or cursor positioning.
func (m *Model) SetHighlighter(h LineHighlighter) {
	m.highlighter = h
}

// highlightRanges returns the highlighter's ranges for a logical line, or
// nil when no highlighter is installed.
func (m *Model) highlightRanges(lineIdx int, line []rune) []lipgloss.Range {
	if m.highlighter == nil {
		return nil
	}
	return m.highlighter.Highlight(lineIdx, line)
}

// styleSegment renders a wrapped-line segment, applying any highlighter
// ranges that intersect it. segStart is the rune offset of the segment
// within its logical line. The cell under the virtual cursor never reaches
// this function: the renderer splits cursor-line content into pre- and
// post-cursor segments and renders the cursor cell itself separately, so
// token styling and cursor styling can never fight over the same cell.
func styleSegment(
	base lipgloss.Style,
	seg []rune,
	segStart int,
	lineRanges []lipgloss.Range,
) string {
	if len(lineRanges) == 0 || len(seg) == 0 {
		return base.Render(string(seg))
	}

	segEnd := segStart + len(seg)
	var ranges []lipgloss.Range
	for _, r := range lineRanges {
		// Intersect the token range with this segment.
		start := max(r.Start, segStart) - segStart
		end := min(r.End, segEnd) - segStart
		if start >= end {
			continue
		}
		ranges = append(ranges, lipgloss.NewRange(start, end, r.Style))
	}

	if len(ranges) == 0 {
		return base.Render(string(seg))
	}

	// StyleRanges composes each range's style over the base-rendered string.
	// Range offsets are 1-based display cells in lipgloss; map rune offsets
	// through display widths to stay correct for double-width runes.
	return lipgloss.StyleRanges(base.Render(string(seg)), toStyleRanges(seg, ranges, base)...)
}

// toStyleRanges converts rune-offset ranges into the display-cell ranges
// lipgloss.StyleRanges expects: zero-based and half-open over the cells of
// the ANSI-stripped string. Each token style inherits the base style's
// attributes so tokens only override what they set.
func toStyleRanges(seg []rune, ranges []lipgloss.Range, base lipgloss.Style) []lipgloss.Range {
	out := make([]lipgloss.Range, 0, len(ranges))
	for _, r := range ranges {
		startCell := runeDisplayWidth(seg[:r.Start])
		endCell := startCell + runeDisplayWidth(seg[r.Start:r.End])
		style := r.Style.Inherit(base)
		out = append(out, lipgloss.NewRange(startCell, endCell, style))
	}
	return out
}

// runeDisplayWidth returns the total display width of runes.
func runeDisplayWidth(r []rune) int {
	w := 0
	for _, ru := range r {
		w += rw.RuneWidth(ru)
	}
	return w
}
