package list

import (
	"image/color"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/core/layout"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/x/ansi"
	"github.com/google/uuid"
	"github.com/rivo/uniseg"
)

type Indexable interface {
	SetIndex(int)
}

type CompletionItem[T any] interface {
	FilterableItem
	layout.Focusable
	layout.Sizeable
	HasMatchIndexes
	Value() T
	Text() string
}

type completionItemCmp[T any] struct {
	width        int
	id           string
	text         string
	value        T
	focus        bool
	matchIndexes []int
	bgColor      color.Color
	shortcut     string
}

type options struct {
	id           string
	text         string
	bgColor      color.Color
	matchIndexes []int
	shortcut     string
}

type CompletionItemOption func(*options)

func WithCompletionBackgroundColor(c color.Color) CompletionItemOption {
	return func(cmp *options) {
		cmp.bgColor = c
	}
}

func WithCompletionMatchIndexes(indexes ...int) CompletionItemOption {
	return func(cmp *options) {
		cmp.matchIndexes = indexes
	}
}

func WithCompletionShortcut(shortcut string) CompletionItemOption {
	return func(cmp *options) {
		cmp.shortcut = shortcut
	}
}

func WithCompletionID(id string) CompletionItemOption {
	return func(cmp *options) {
		cmp.id = id
	}
}

func NewCompletionItem[T any](text string, value T, opts ...CompletionItemOption) CompletionItem[T] {
	c := &completionItemCmp[T]{
		text:  text,
		value: value,
	}
	o := &options{}

	for _, opt := range opts {
		opt(o)
	}
	if o.id == "" {
		o.id = uuid.NewString()
	}
	c.id = o.id
	c.bgColor = o.bgColor
	c.matchIndexes = o.matchIndexes
	c.shortcut = o.shortcut
	return c
}

// Init implements CommandItem.
func (c *completionItemCmp[T]) Init() tea.Cmd {
	return nil
}

// Update implements CommandItem.
func (c *completionItemCmp[T]) Update(tea.Msg) (util.Model, tea.Cmd) {
	return c, nil
}

// View implements CommandItem.
func (c *completionItemCmp[T]) View() string {
	t := styles.CurrentTheme()

	itemStyle := t.S().Base.Padding(0, 1).Width(c.width)
	innerWidth := c.width - 2 // Account for padding

	if c.shortcut != "" {
		innerWidth -= lipgloss.Width(c.shortcut)
	}

	titleStyle := t.S().Text.Width(innerWidth)
	titleMatchStyle := t.S().Text.Underline(true)
	if c.bgColor != nil {
		titleStyle = titleStyle.Background(c.bgColor)
		titleMatchStyle = titleMatchStyle.Background(c.bgColor)
		itemStyle = itemStyle.Background(c.bgColor)
	}

	if c.focus {
		titleStyle = t.S().TextSelected.Width(innerWidth)
		titleMatchStyle = t.S().TextSelected.Underline(true)
		itemStyle = itemStyle.Background(t.Primary)
	}

	var truncatedTitle string

	if len(c.matchIndexes) > 0 && len(c.text) > innerWidth {
		// Smart truncation: ensure the last matching part is visible
		truncatedTitle = c.smartTruncate(c.text, innerWidth, c.matchIndexes)
	} else {
		// No matches, use regular truncation
		truncatedTitle = ansi.Truncate(c.text, innerWidth, "…")
	}

	text := titleStyle.Render(truncatedTitle)
	if len(c.matchIndexes) > 0 {
		var ranges []lipgloss.Range
		for _, rng := range matchedRanges(c.matchIndexes) {
			// ansi.Cut is grapheme and ansi sequence aware, we match against a ansi.Stripped string, but we might still have graphemes.
			// all that to say that rng is byte positions, but we need to pass it down to ansi.Cut as char positions.
			// so we need to adjust it here:
			start, stop := bytePosToVisibleCharPos(truncatedTitle, rng)
			ranges = append(ranges, lipgloss.NewRange(start, stop+1, titleMatchStyle))
		}
		text = lipgloss.StyleRanges(text, ranges...)
	}
	parts := []string{text}
	if c.shortcut != "" {
		// Add the shortcut at the end
		shortcutStyle := t.S().Muted
		if c.focus {
			shortcutStyle = t.S().TextSelected
		}
		parts = append(parts, shortcutStyle.Render(c.shortcut))
	}
	item := itemStyle.Render(
		lipgloss.JoinHorizontal(
			lipgloss.Left,
			parts...,
		),
	)
	return item
}

// Blur implements CommandItem.
func (c *completionItemCmp[T]) Blur() tea.Cmd {
	c.focus = false
	return nil
}

// Focus implements CommandItem.
func (c *completionItemCmp[T]) Focus() tea.Cmd {
	c.focus = true
	return nil
}

// GetSize implements CommandItem.
func (c *completionItemCmp[T]) GetSize() (int, int) {
	return c.width, 1
}

// IsFocused implements CommandItem.
func (c *completionItemCmp[T]) IsFocused() bool {
	return c.focus
}

// SetSize implements CommandItem.
func (c *completionItemCmp[T]) SetSize(width int, height int) tea.Cmd {
	c.width = width
	return nil
}

func (c *completionItemCmp[T]) MatchIndexes(indexes []int) {
	c.matchIndexes = indexes
}

func (c *completionItemCmp[T]) FilterValue() string {
	return c.text
}

func (c *completionItemCmp[T]) Value() T {
	return c.value
}

// smartTruncate implements fzf-style truncation that ensures the last matching part is visible
func (c *completionItemCmp[T]) smartTruncate(text string, width int, matchIndexes []int) string {
	if width <= 0 {
		return ""
	}

	textLen := ansi.StringWidth(text)
	if textLen <= width {
		return text
	}

	if len(matchIndexes) == 0 {
		return ansi.Truncate(text, width, "…")
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
		return ansi.Truncate(text, width, "…")
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
	return truncatedText
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

// ID implements CompletionItem.
func (c *completionItemCmp[T]) ID() string {
	return c.id
}

func (c *completionItemCmp[T]) Text() string {
	return c.text
}

type ItemSection interface {
	Item
	layout.Sizeable
	Indexable
	SetInfo(info string)
	Title() string
}
type itemSectionModel struct {
	width int
	title string
	inx   int
	id    string
	info  string
}

// ID implements ItemSection.
func (m *itemSectionModel) ID() string {
	return m.id
}

// Title implements ItemSection.
func (m *itemSectionModel) Title() string {
	return m.title
}

func NewItemSection(title string) ItemSection {
	return &itemSectionModel{
		title: title,
		inx:   -1,
		id:    uuid.NewString(),
	}
}

func (m *itemSectionModel) Init() tea.Cmd {
	return nil
}

func (m *itemSectionModel) Update(tea.Msg) (util.Model, tea.Cmd) {
	return m, nil
}

func (m *itemSectionModel) View() string {
	t := styles.CurrentTheme()
	title := ansi.Truncate(m.title, m.width-2, "…")
	style := t.S().Base.Padding(1, 1, 0, 1)
	if m.inx == 0 {
		style = style.Padding(0, 1, 0, 1)
	}
	title = t.S().Muted.Render(title)
	section := ""
	if m.info != "" {
		section = core.SectionWithInfo(title, m.width-2, m.info)
	} else {
		section = core.Section(title, m.width-2)
	}

	return style.Render(section)
}

func (m *itemSectionModel) GetSize() (int, int) {
	return m.width, 1
}

func (m *itemSectionModel) SetSize(width int, height int) tea.Cmd {
	m.width = width
	return nil
}

func (m *itemSectionModel) IsSectionHeader() bool {
	return true
}

func (m *itemSectionModel) SetInfo(info string) {
	m.info = info
}

func (m *itemSectionModel) SetIndex(inx int) {
	m.inx = inx
}
