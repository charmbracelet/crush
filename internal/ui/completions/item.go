package completions

import (
	"slices"
	"strings"

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

// ResourceCompletionValue represents a MCP resource completion value.
type ResourceCompletionValue struct {
	MCPName  string
	URI      string
	Title    string
	MIMEType string
}

// SkillCompletionValue represents an agent skill completion value. The
// fields mirror the SKILL.md frontmatter: Description is shown after the
// name in the popup and folded into the item's filter text, so a skill is
// findable by what it does and not only by what it is called.
type SkillCompletionValue struct {
	Name        string
	Description string
	Path        string
}

// IsTemplate reports whether the completion's URI is an unexpanded RFC 6570
// URI template (it still contains a "{expression}") rather than a concrete,
// readable resource URI. Templates come from resources/templates/list and
// cannot be read until their placeholders are filled in.
func (r ResourceCompletionValue) IsTemplate() bool {
	return strings.Contains(r.URI, "{")
}

// CompletionItem represents an item in the completions list.
type CompletionItem struct {
	*list.Versioned

	text    string
	value   any
	match   fuzzy.Match
	focused bool
	cache   map[int]string

	// detailStart is the byte offset in text where the secondary detail
	// (e.g. a skill's description) begins, or -1 when the item is all
	// primary text. The detail renders dimmed so the name still leads.
	detailStart int

	// sortKey is what the name-priority tiering ranks on. It defaults to
	// text, but items whose display text carries a trailing detail set it
	// to the bare name so the description can't skew the tier.
	sortKey string

	// Styles
	normalStyle  lipgloss.Style
	focusedStyle lipgloss.Style
	matchStyle   lipgloss.Style
}

// NewCompletionItem creates a new completion item.
func NewCompletionItem(text string, value any, normalStyle, focusedStyle, matchStyle lipgloss.Style) *CompletionItem {
	return &CompletionItem{
		Versioned:    list.NewVersioned(),
		text:         text,
		value:        value,
		detailStart:  -1,
		sortKey:      text,
		normalStyle:  normalStyle,
		focusedStyle: focusedStyle,
		matchStyle:   matchStyle,
	}
}

// withDetail marks everything from byte offset start onwards as secondary
// detail text and ranks the item on sortKey instead of its full text.
func (c *CompletionItem) withDetail(start int, sortKey string) *CompletionItem {
	c.detailStart = start
	c.sortKey = sortKey
	return c
}

// SortKey returns the string the name-priority tiering ranks on.
func (c *CompletionItem) SortKey() string {
	return c.sortKey
}

// Finished implements list.Item. Completion items render purely from
// (text, match, focus); any mutation (SetMatch / SetFocused) bumps
// Version() so the frozen cache entry invalidates on the next
// render. Marking them finished lets the F6 list memo skip the
// per-line work for the steady completions popup.
func (c *CompletionItem) Finished() bool {
	return true
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
	if sameFuzzyMatch(c.match, m) {
		return
	}
	c.cache = nil
	c.match = m
	c.Bump()
}

// sameFuzzyMatch reports whether two fuzzy.Match values are
// observably equal. Because Match contains a slice (MatchedIndexes)
// it is not directly comparable with ==; we compare the scalar
// fields and then walk the indexes. SetMatch uses this to skip
// gratuitous version bumps when the same match is reapplied.
func sameFuzzyMatch(a, b fuzzy.Match) bool {
	return a.Str == b.Str &&
		a.Index == b.Index &&
		a.Score == b.Score &&
		slices.Equal(a.MatchedIndexes, b.MatchedIndexes)
}

// SetFocused implements [list.Focusable].
func (c *CompletionItem) SetFocused(focused bool) {
	if c.focused == focused {
		return
	}
	c.cache = nil
	c.focused = focused
	c.Bump()
}

// Render implements [list.Item].
func (c *CompletionItem) Render(width int) string {
	return renderItem(
		c.normalStyle,
		c.focusedStyle,
		c.matchStyle,
		c.text,
		c.detailStart,
		c.focused,
		width,
		c.cache,
		&c.match,
	)
}

func renderItem(
	normalStyle, focusedStyle, matchStyle lipgloss.Style,
	text string,
	detailStart int,
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

	// Dim the trailing detail (a skill's description) so the name still
	// leads the row. Applied before the match ranges so a fuzzy hit inside
	// the description still reads as a match.
	if detailStart >= 0 {
		start, _ := bytePosToVisibleCharPos(text, [2]int{detailStart, detailStart})
		if start+1 < width {
			detailStyle := style.Faint(true)
			content = lipgloss.StyleRanges(content, lipgloss.NewRange(start+1, width, detailStyle))
		}
	}

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
