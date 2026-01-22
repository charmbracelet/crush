package completions

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/x/ansi"
	"github.com/rivo/uniseg"
	"github.com/sahilm/fuzzy"
)

// FileCompletionValue represents a file path completion value.
type FileCompletionValue struct {
	Path string
}

// CompletionItem represents an item in the completions list.
type CompletionItem struct {
	text    string
	value   any
	match   fuzzy.Match
	focused bool
	cache   map[int]string

	// Styles
	normalStyle  lipgloss.Style
	focusedStyle lipgloss.Style
	matchStyle   lipgloss.Style
}

// NewCompletionItem creates a new completion item.
func NewCompletionItem(text string, value any, normalStyle, focusedStyle, matchStyle lipgloss.Style) *CompletionItem {
	return &CompletionItem{
		text:         text,
		value:        value,
		normalStyle:  normalStyle,
		focusedStyle: focusedStyle,
		matchStyle:   matchStyle,
	}
}

// Text returns the display text of the item.
func (c *CompletionItem) Text() string {
	return c.text
}

// Value returns the value of the item.
func (c *CompletionItem) Value() any {
	return c.value
}

// Filter implements [list.FilterableItem].
func (c *CompletionItem) Filter() string {
	return c.text
}

// SetMatch implements [list.MatchSettable].
func (c *CompletionItem) SetMatch(m fuzzy.Match) {
	c.cache = nil
	c.match = m
}

// SetFocused implements [list.Focusable].
func (c *CompletionItem) SetFocused(focused bool) {
	if c.focused != focused {
		c.cache = nil
	}
	c.focused = focused
}

// Render implements [list.Item].
func (c *CompletionItem) Render(width int) string {
	return renderItem(
		c.normalStyle,
		c.focusedStyle,
		c.matchStyle,
		c.text,
		c.focused,
		width,
		c.cache,
		&c.match,
	)
}

func renderItem(
	normalStyle, focusedStyle, matchStyle lipgloss.Style,
	text string,
	focused bool,
	width int,
	cache map[int]string,
	match *fuzzy.Match,
) string {
	if cache == nil {
		cache = make(map[int]string)
	}

	cached, ok := cache[width]
	if ok {
		return cached
	}

	innerWidth := width - 2 // Account for padding
	// Truncate if needed.
	if ansi.StringWidth(text) > innerWidth {
		text = ansi.Truncate(text, innerWidth, "…")
	}

	// Select base style.
	style := normalStyle
	matchStyle = matchStyle.Background(style.GetBackground())
	if focused {
		style = focusedStyle
		matchStyle = matchStyle.Background(style.GetBackground())
	}

	// Render full-width text with background.
	content := style.Padding(0, 1).Width(width).Render(text)

	// Apply match highlighting using StyleRanges.
	if len(match.MatchedIndexes) > 0 {
		var ranges []lipgloss.Range
		for _, rng := range matchedRanges(match.MatchedIndexes) {
			start, stop := bytePosToVisibleCharPos(text, rng)
			// Offset by 1 for the padding space.
			ranges = append(ranges, lipgloss.NewRange(start+1, stop+2, matchStyle))
		}
		content = lipgloss.StyleRanges(content, ranges...)
	}

	cache[width] = content
	return content
}

// matchedRanges converts a list of match indexes into contiguous ranges.
func matchedRanges(in []int) [][2]int {
	if len(in) == 0 {
		return [][2]int{}
	}
	current := [2]int{in[0], in[0]}
	if len(in) == 1 {
		return [][2]int{current}
	}
	var out [][2]int
	for i := 1; i < len(in); i++ {
		if in[i] == current[1]+1 {
			current[1] = in[i]
		} else {
			out = append(out, current)
			current = [2]int{in[i], in[i]}
		}
	}
	out = append(out, current)
	return out
}

// bytePosToVisibleCharPos converts byte positions to visible character positions.
func bytePosToVisibleCharPos(str string, rng [2]int) (int, int) {
	bytePos, byteStart, byteStop := 0, rng[0], rng[1]
	pos, start, stop := 0, 0, 0
	gr := uniseg.NewGraphemes(str)
	for byteStart > bytePos {
		if !gr.Next() {
			break
		}
		bytePos += len(gr.Str())
		pos += max(1, gr.Width())
	}
	start = pos
	for byteStop > bytePos {
		if !gr.Next() {
			break
		}
		bytePos += len(gr.Str())
		pos += max(1, gr.Width())
	}
	stop = pos
	return start, stop
}

// Ensure CompletionItem implements the required interfaces.
var (
	_ list.Item           = (*CompletionItem)(nil)
	_ list.FilterableItem = (*CompletionItem)(nil)
	_ list.MatchSettable  = (*CompletionItem)(nil)
	_ list.Focusable      = (*CompletionItem)(nil)
)
