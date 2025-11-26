package list

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/glamour/v2"
	"github.com/charmbracelet/glamour/v2/ansi"
	uv "github.com/charmbracelet/ultraviolet"
)

// Item represents a list item that can draw itself to a UV buffer.
// Items implement the uv.Drawable interface.
type Item interface {
	uv.Drawable

	// ID returns unique identifier for this item.
	ID() string

	// Height returns the item's height in lines for the given width.
	// This allows items to calculate height based on text wrapping and available space.
	Height(width int) int
}

// Focusable is an optional interface for items that support focus.
// When implemented, items can change appearance when focused (borders, colors, etc).
type Focusable interface {
	Focus()
	Blur()
	IsFocused() bool
}

// StringItem is a simple string-based item with optional text wrapping.
// It caches rendered content by width for efficient repeated rendering.
type StringItem struct {
	id      string
	content string // Raw content string (may contain ANSI styles)
	wrap    bool   // Whether to wrap text

	// Cache for rendered content at specific widths
	// Key: width, Value: string
	cache map[int]string
}

// NewStringItem creates a new string item with the given ID and content.
func NewStringItem(id, content string) *StringItem {
	return &StringItem{
		id:      id,
		content: content,
		wrap:    false,
		cache:   make(map[int]string),
	}
}

// NewWrappingStringItem creates a new string item that wraps text to fit width.
func NewWrappingStringItem(id, content string) *StringItem {
	return &StringItem{
		id:      id,
		content: content,
		wrap:    true,
		cache:   make(map[int]string),
	}
}

// ID implements Item.
func (s *StringItem) ID() string {
	return s.id
}

// Height implements Item.
func (s *StringItem) Height(width int) int {
	if !s.wrap {
		// No wrapping - height is just the number of newlines + 1
		return strings.Count(s.content, "\n") + 1
	}

	// Use lipgloss.Wrap to wrap the content and count lines
	// This preserves ANSI styles and is much faster than rendering to a buffer
	wrapped := lipgloss.Wrap(s.content, width, "")
	return strings.Count(wrapped, "\n") + 1
}

// Draw implements Item and uv.Drawable.
func (s *StringItem) Draw(scr uv.Screen, area uv.Rectangle) {
	width := area.Dx()

	// Check cache first
	content, ok := s.cache[width]
	if !ok {
		// Not cached - create and cache
		content = s.content
		if s.wrap {
			// Wrap content using lipgloss
			content = lipgloss.Wrap(s.content, width, "")
		}
		s.cache[width] = content
	}

	// Draw the cached styled string
	styled := uv.NewStyledString(content)
	styled.Draw(scr, area)
}

// MarkdownItem renders markdown content using Glamour.
// It caches all rendered content by width for efficient repeated rendering.
// The wrap width is capped at 120 cells by default to ensure readable line lengths.
// MarkdownItem implements Focusable if focusStyle and blurStyle are not nil.
type MarkdownItem struct {
	id          string
	markdown    string            // Raw markdown content
	styleConfig *ansi.StyleConfig // Optional style configuration
	maxWidth    int               // Maximum wrap width (default 120)
	focused     bool              // Current focus state
	focusStyle  *lipgloss.Style   // Optional focus style
	blurStyle   *lipgloss.Style   // Optional blur style

	// Cache for rendered content at specific widths
	// Key: width (capped to maxWidth), Value: rendered markdown string
	cache map[int]string
}

// DefaultMarkdownMaxWidth is the default maximum width for markdown rendering.
const DefaultMarkdownMaxWidth = 120

// NewMarkdownItem creates a new markdown item with the given ID and markdown content.
// If focusStyle and blurStyle are both non-nil, the item will implement Focusable.
func NewMarkdownItem(id, markdown string) *MarkdownItem {
	m := &MarkdownItem{
		id:       id,
		markdown: markdown,
		maxWidth: DefaultMarkdownMaxWidth,
		cache:    make(map[int]string),
	}

	return m
}

// WithStyleConfig sets a custom Glamour style configuration for the markdown item.
func (m *MarkdownItem) WithStyleConfig(styleConfig ansi.StyleConfig) *MarkdownItem {
	m.styleConfig = &styleConfig
	return m
}

// WithMaxWidth sets the maximum wrap width for markdown rendering.
func (m *MarkdownItem) WithMaxWidth(maxWidth int) *MarkdownItem {
	m.maxWidth = maxWidth
	return m
}

// WithFocusStyles sets the focus and blur styles for the markdown item.
// If both styles are non-nil, the item will implement Focusable.
func (m *MarkdownItem) WithFocusStyles(focusStyle, blurStyle *lipgloss.Style) *MarkdownItem {
	m.focusStyle = focusStyle
	m.blurStyle = blurStyle
	return m
}

// ID implements Item.
func (m *MarkdownItem) ID() string {
	return m.id
}

// Height implements Item.
func (m *MarkdownItem) Height(width int) int {
	// Render the markdown to get its height
	rendered := m.renderMarkdown(width)

	// Apply focus/blur styling if configured to get accurate height
	if m.focusStyle != nil && m.blurStyle != nil {
		var style lipgloss.Style
		if m.focused {
			style = *m.focusStyle
		} else {
			style = *m.blurStyle
		}
		rendered = style.Render(rendered)
	}

	return strings.Count(rendered, "\n") + 1
}

// Draw implements Item and uv.Drawable.
func (m *MarkdownItem) Draw(scr uv.Screen, area uv.Rectangle) {
	width := area.Dx()
	rendered := m.renderMarkdown(width)

	// Apply focus/blur styling if configured
	if m.focusStyle != nil && m.blurStyle != nil {
		var style lipgloss.Style
		if m.focused {
			style = *m.focusStyle
		} else {
			style = *m.blurStyle
		}
		rendered = style.Render(rendered)
	}

	// Draw the rendered markdown
	styled := uv.NewStyledString(rendered)
	styled.Draw(scr, area)
}

// renderMarkdown renders the markdown content at the given width, using cache if available.
// Width is always capped to maxWidth to ensure readable line lengths.
func (m *MarkdownItem) renderMarkdown(width int) string {
	// Cap width to maxWidth
	cappedWidth := min(width, m.maxWidth)

	// Check cache first (always cache all rendered markdown)
	if cached, ok := m.cache[cappedWidth]; ok {
		return cached
	}

	// Not cached - render now
	opts := []glamour.TermRendererOption{
		glamour.WithWordWrap(cappedWidth),
	}

	// Add style config if provided
	if m.styleConfig != nil {
		opts = append(opts, glamour.WithStyles(*m.styleConfig))
	}

	renderer, err := glamour.NewTermRenderer(opts...)
	if err != nil {
		// Fallback to plain text on error
		return m.markdown
	}

	rendered, err := renderer.Render(m.markdown)
	if err != nil {
		// Fallback to plain text on error
		return m.markdown
	}

	// Trim trailing whitespace
	rendered = strings.TrimRight(rendered, "\n\r ")

	// Always cache
	m.cache[cappedWidth] = rendered

	return rendered
}

// Focus implements Focusable interface.
// Only works if both focusStyle and blurStyle are non-nil.
func (m *MarkdownItem) Focus() {
	if m.focusStyle != nil && m.blurStyle != nil {
		m.focused = true
	}
}

// Blur implements Focusable interface.
// Only works if both focusStyle and blurStyle are non-nil.
func (m *MarkdownItem) Blur() {
	if m.focusStyle != nil && m.blurStyle != nil {
		m.focused = false
	}
}

// IsFocused implements Focusable interface.
// Returns false if styles are not configured.
func (m *MarkdownItem) IsFocused() bool {
	if m.focusStyle == nil || m.blurStyle == nil {
		return false
	}
	return m.focused
}

// FocusableItem wraps another Item to provide focus behavior with optional
// Lip Gloss styles. If styles are nil, the item is drawn without additional styling.
type FocusableItem struct {
	item       Item
	focused    bool
	focusStyle *lipgloss.Style
	blurStyle  *lipgloss.Style
}

var (
	_ Item      = (*FocusableItem)(nil)
	_ Focusable = (*FocusableItem)(nil)
)

// NewFocusableItem creates a new FocusableItem wrapping the given item with
// optional focus and blur styles. Pass nil for either style to disable styling.
func NewFocusableItem(item Item, focusStyle, blurStyle *lipgloss.Style) *FocusableItem {
	return &FocusableItem{
		item:       item,
		focusStyle: focusStyle,
		blurStyle:  blurStyle,
	}
}

// ID implements Item.
func (f *FocusableItem) ID() string {
	return f.item.ID()
}

// Height implements Item.
// Returns the height including the frame size from the current style.
func (f *FocusableItem) Height(width int) int {
	style := f.blurStyle
	if f.focused {
		style = f.focusStyle
	}

	// If no style, return inner item height directly
	if style == nil {
		return f.item.Height(width)
	}

	vFrameSize := style.GetVerticalFrameSize()
	hFrameSize := style.GetHorizontalFrameSize()

	// Calculate inner width after accounting for horizontal frame
	innerWidth := width
	if hFrameSize > 0 {
		innerWidth -= hFrameSize
	}

	// Get inner item height and add vertical frame
	innerHeight := f.item.Height(innerWidth)
	return innerHeight + vFrameSize
}

// Draw implements Item and uv.Drawable.
func (f *FocusableItem) Draw(scr uv.Screen, area uv.Rectangle) {
	style := f.blurStyle
	if f.focused {
		style = f.focusStyle
	}

	// If no style, draw inner item directly
	if style == nil {
		f.item.Draw(scr, area)
		return
	}

	// Get the size occupied by border and padding
	vFrameSize := style.GetVerticalFrameSize()
	hFrameSize := style.GetHorizontalFrameSize()

	// Calculate inner content size (area minus frame)
	innerWidth := area.Dx() - hFrameSize
	innerHeight := area.Dy() - vFrameSize

	// Render inner item to buffer
	innerBuf := uv.NewScreenBuffer(innerWidth, innerHeight)
	f.item.Draw(&innerBuf, uv.Rect(0, 0, innerWidth, innerHeight))
	innerContent := innerBuf.Render()

	// Apply border+padding style to the rendered content
	bordered := style.Width(area.Dx()).Render(innerContent)
	styled := uv.NewStyledString(bordered)
	styled.Draw(scr, area)
}

// Focus implements Focusable.
func (f *FocusableItem) Focus() {
	f.focused = true
}

// Blur implements Focusable.
func (f *FocusableItem) Blur() {
	f.focused = false
}

// IsFocused implements Focusable.
func (f *FocusableItem) IsFocused() bool {
	return f.focused
}

// Gap is a 1-line spacer item used to add gaps between items.
var Gap = NewSpacerItem("spacer-gap", 1)

// SpacerItem is an empty item that takes up vertical space.
// Useful for adding gaps between items in a list.
type SpacerItem struct {
	id     string
	height int
}

var _ Item = (*SpacerItem)(nil)

// NewSpacerItem creates a new spacer item with the given ID and height in lines.
func NewSpacerItem(id string, height int) *SpacerItem {
	return &SpacerItem{
		id:     id,
		height: height,
	}
}

// ID implements Item.
func (s *SpacerItem) ID() string {
	return s.id
}

// Height implements Item.
func (s *SpacerItem) Height(width int) int {
	return s.height
}

// Draw implements Item.
// Spacer items don't draw anything, they just take up space.
func (s *SpacerItem) Draw(scr uv.Screen, area uv.Rectangle) {
	// Intentionally empty - spacers are invisible
}
