package list

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/glamour/v2"
	"github.com/charmbracelet/glamour/v2/ansi"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/ultraviolet/screen"
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

// BaseFocusable provides common focus state and styling for items.
// Embed this type to add focus behavior to any item.
type BaseFocusable struct {
	focused    bool
	focusStyle *lipgloss.Style
	blurStyle  *lipgloss.Style
}

// Focus implements Focusable interface.
func (b *BaseFocusable) Focus() {
	b.focused = true
}

// Blur implements Focusable interface.
func (b *BaseFocusable) Blur() {
	b.focused = false
}

// IsFocused implements Focusable interface.
func (b *BaseFocusable) IsFocused() bool {
	return b.focused
}

// HasFocusStyles returns true if both focus and blur styles are configured.
func (b *BaseFocusable) HasFocusStyles() bool {
	return b.focusStyle != nil && b.blurStyle != nil
}

// CurrentStyle returns the current style based on focus state.
// Returns nil if no styles are configured, or if the current state's style is nil.
func (b *BaseFocusable) CurrentStyle() *lipgloss.Style {
	if b.focused {
		return b.focusStyle
	}
	return b.blurStyle
}

// SetFocusStyles sets the focus and blur styles.
func (b *BaseFocusable) SetFocusStyles(focusStyle, blurStyle *lipgloss.Style) {
	b.focusStyle = focusStyle
	b.blurStyle = blurStyle
}

// StringItem is a simple string-based item with optional text wrapping.
// It caches rendered content by width for efficient repeated rendering.
// StringItem implements Focusable if focusStyle and blurStyle are set via WithFocusStyles.
type StringItem struct {
	BaseFocusable
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

// WithFocusStyles sets the focus and blur styles for the string item.
// If both styles are non-nil, the item will implement Focusable.
func (s *StringItem) WithFocusStyles(focusStyle, blurStyle *lipgloss.Style) *StringItem {
	s.SetFocusStyles(focusStyle, blurStyle)
	return s
}

// ID implements Item.
func (s *StringItem) ID() string {
	return s.id
}

// Height implements Item.
func (s *StringItem) Height(width int) int {
	// Calculate content width if we have styles
	contentWidth := width
	if style := s.CurrentStyle(); style != nil {
		hFrameSize := style.GetHorizontalFrameSize()
		if hFrameSize > 0 {
			contentWidth -= hFrameSize
		}
	}

	var lines int
	if !s.wrap {
		// No wrapping - height is just the number of newlines + 1
		lines = strings.Count(s.content, "\n") + 1
	} else {
		// Use lipgloss.Wrap to wrap the content and count lines
		// This preserves ANSI styles and is much faster than rendering to a buffer
		wrapped := lipgloss.Wrap(s.content, contentWidth, "")
		lines = strings.Count(wrapped, "\n") + 1
	}

	// Add vertical frame size if we have styles
	if style := s.CurrentStyle(); style != nil {
		lines += style.GetVerticalFrameSize()
	}

	return lines
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

	// Apply focus/blur styling if configured
	if style := s.CurrentStyle(); style != nil {
		content = style.Width(width).Render(content)
	}

	// Draw the styled string
	styled := uv.NewStyledString(content)
	styled.Draw(scr, area)
}

// MarkdownItem renders markdown content using Glamour.
// It caches all rendered content by width for efficient repeated rendering.
// The wrap width is capped at 120 cells by default to ensure readable line lengths.
// MarkdownItem implements Focusable if focusStyle and blurStyle are set via WithFocusStyles.
type MarkdownItem struct {
	BaseFocusable
	id          string
	markdown    string            // Raw markdown content
	styleConfig *ansi.StyleConfig // Optional style configuration
	maxWidth    int               // Maximum wrap width (default 120)

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
	m.SetFocusStyles(focusStyle, blurStyle)
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
	if style := m.CurrentStyle(); style != nil {
		rendered = style.Render(rendered)
	}

	return strings.Count(rendered, "\n") + 1
}

// Draw implements Item and uv.Drawable.
func (m *MarkdownItem) Draw(scr uv.Screen, area uv.Rectangle) {
	width := area.Dx()
	rendered := m.renderMarkdown(width)

	// Apply focus/blur styling if configured
	if style := m.CurrentStyle(); style != nil {
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
	// Ensure the area is filled with spaces to clear any existing content
	spacerArea := uv.Rect(area.Min.X, area.Min.Y, area.Dx(), area.Min.Y+min(1, s.height))
	if spacerArea.Overlaps(area) {
		screen.ClearArea(scr, spacerArea)
	}
}
