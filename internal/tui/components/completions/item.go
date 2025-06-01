package completions

import (
	"image/color"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/opencode-ai/opencode/internal/tui/components/core/list"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/util"
	"github.com/rivo/uniseg"
)

type CompletionItem interface {
	util.Model
	layout.Focusable
	layout.Sizeable
	list.HasMatchIndexes
	list.HasFilterValue
	Value() any
}

type completionItemCmp struct {
	width        int
	text         string
	value        any
	focus        bool
	matchIndexes []int
	bgColor      color.Color
}

type completionOptions func(*completionItemCmp)

func WithBackgroundColor(c color.Color) completionOptions {
	return func(cmp *completionItemCmp) {
		cmp.bgColor = c
	}
}

func WithMatchIndexes(indexes ...int) completionOptions {
	return func(cmp *completionItemCmp) {
		cmp.matchIndexes = indexes
	}
}

func NewCompletionItem(text string, value any, opts ...completionOptions) CompletionItem {
	c := &completionItemCmp{
		text:  text,
		value: value,
	}

	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Init implements CommandItem.
func (c *completionItemCmp) Init() tea.Cmd {
	return nil
}

// Update implements CommandItem.
func (c *completionItemCmp) Update(tea.Msg) (tea.Model, tea.Cmd) {
	return c, nil
}

// View implements CommandItem.
func (c *completionItemCmp) View() tea.View {
	t := styles.CurrentTheme()

	titleStyle := t.S().Text.Padding(0, 1).Width(c.width)
	titleMatchStyle := t.S().Text.Underline(true)
	if c.bgColor != nil {
		titleStyle = titleStyle.Background(c.bgColor)
		titleMatchStyle = titleMatchStyle.Background(c.bgColor)
	}

	if c.focus {
		titleStyle = t.S().TextSelected.Padding(0, 1).Width(c.width)
		titleMatchStyle = t.S().TextSelected.Underline(true)
	}

	var truncatedTitle string
	var adjustedMatchIndexes []int

	availableWidth := c.width - 2 // Account for padding
	if len(c.matchIndexes) > 0 && len(c.text) > availableWidth {
		// Smart truncation: ensure the last matching part is visible
		truncatedTitle, adjustedMatchIndexes = c.smartTruncate(c.text, availableWidth, c.matchIndexes)
	} else {
		// No matches, use regular truncation
		truncatedTitle = ansi.Truncate(c.text, availableWidth, "…")
		adjustedMatchIndexes = c.matchIndexes
	}

	text := titleStyle.Render(truncatedTitle)
	if len(adjustedMatchIndexes) > 0 {
		var ranges []lipgloss.Range
		for _, rng := range matchedRanges(adjustedMatchIndexes) {
			// ansi.Cut is grapheme and ansi sequence aware, we match against a ansi.Stripped string, but we might still have graphemes.
			// all that to say that rng is byte positions, but we need to pass it down to ansi.Cut as char positions.
			// so we need to adjust it here:
			start, stop := bytePosToVisibleCharPos(text, rng)
			ranges = append(ranges, lipgloss.NewRange(start, stop+1, titleMatchStyle))
		}
		text = lipgloss.StyleRanges(text, ranges...)
	}
	return tea.NewView(text)
}

// Blur implements CommandItem.
func (c *completionItemCmp) Blur() tea.Cmd {
	c.focus = false
	return nil
}

// Focus implements CommandItem.
func (c *completionItemCmp) Focus() tea.Cmd {
	c.focus = true
	return nil
}

// GetSize implements CommandItem.
func (c *completionItemCmp) GetSize() (int, int) {
	return c.width, 1
}

// IsFocused implements CommandItem.
func (c *completionItemCmp) IsFocused() bool {
	return c.focus
}

// SetSize implements CommandItem.
func (c *completionItemCmp) SetSize(width int, height int) tea.Cmd {
	c.width = width
	return nil
}

func (c *completionItemCmp) MatchIndexes(indexes []int) {
	c.matchIndexes = indexes
	for i := range c.matchIndexes {
		c.matchIndexes[i] += 1 // Adjust for the padding we add in View
	}
}

func (c *completionItemCmp) FilterValue() string {
	return c.text
}

func (c *completionItemCmp) Value() any {
	return c.value
}

// smartTruncate implements fzf-style truncation that ensures the last matching part is visible
func (c *completionItemCmp) smartTruncate(text string, width int, matchIndexes []int) (string, []int) {
	if width <= 0 {
		return "", []int{}
	}

	textLen := ansi.StringWidth(text)
	if textLen <= width {
		return text, matchIndexes
	}

	if len(matchIndexes) == 0 {
		return ansi.Truncate(text, width, "…"), []int{}
	}

	// Find the last match position
	lastMatchPos := matchIndexes[len(matchIndexes)-1]

	// Convert byte position to visual width position
	lastMatchVisualPos := 0
	bytePos := 0
	gr := uniseg.NewGraphemes(text)
	for bytePos < lastMatchPos && gr.Next() {
		bytePos += len(gr.Str())
		lastMatchVisualPos += max(1, gr.Width())
	}

	// Calculate how much space we need for the ellipsis
	ellipsisWidth := 1 // "…" character width
	availableWidth := width - ellipsisWidth

	// If the last match is within the available width, truncate from the end
	if lastMatchVisualPos < availableWidth {
		return ansi.Truncate(text, width, "…"), matchIndexes
	}

	// Calculate the start position to ensure the last match is visible
	// We want to show some context before the last match if possible
	startVisualPos := max(0, lastMatchVisualPos-availableWidth+1)

	// Convert visual position back to byte position
	startBytePos := 0
	currentVisualPos := 0
	gr = uniseg.NewGraphemes(text)
	for currentVisualPos < startVisualPos && gr.Next() {
		startBytePos += len(gr.Str())
		currentVisualPos += max(1, gr.Width())
	}

	// Extract the substring starting from startBytePos
	truncatedText := text[startBytePos:]

	// Truncate to fit width with ellipsis
	truncatedText = ansi.Truncate(truncatedText, availableWidth, "")
	truncatedText = "…" + truncatedText

	// Adjust match indexes for the new truncated string
	adjustedIndexes := []int{}
	for _, idx := range matchIndexes {
		if idx >= startBytePos {
			newIdx := idx - startBytePos + 1 //
			// Check if this match is still within the truncated string
			if newIdx < len(truncatedText) {
				adjustedIndexes = append(adjustedIndexes, newIdx)
			}
		}
	}

	return truncatedText, adjustedIndexes
}

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
