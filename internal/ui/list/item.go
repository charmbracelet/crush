package list

import (
	"image"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/glamour/v2"
	"github.com/charmbracelet/glamour/v2/ansi"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/ultraviolet/screen"
)

// toUVStyle converts a lipgloss.Style to a uv.Style, stripping multiline attributes.
func toUVStyle(lgStyle lipgloss.Style) uv.Style {
	var uvStyle uv.Style

	// Colors are already color.Color
	uvStyle.Fg = lgStyle.GetForeground()
	uvStyle.Bg = lgStyle.GetBackground()

	// Build attributes using bitwise OR
	var attrs uint8

	if lgStyle.GetBold() {
		attrs |= uv.AttrBold
	}

	if lgStyle.GetItalic() {
		attrs |= uv.AttrItalic
	}

	if lgStyle.GetUnderline() {
		uvStyle.Underline = uv.UnderlineSingle
	}

	if lgStyle.GetStrikethrough() {
		attrs |= uv.AttrStrikethrough
	}

	if lgStyle.GetFaint() {
		attrs |= uv.AttrFaint
	}

	if lgStyle.GetBlink() {
		attrs |= uv.AttrBlink
	}

	if lgStyle.GetReverse() {
		attrs |= uv.AttrReverse
	}

	uvStyle.Attrs = attrs

	return uvStyle
}

// Item represents a list item that can draw itself to a UV buffer.
// Items implement the uv.Drawable interface.
type Item interface {
	uv.Drawable

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

// Highlightable is an optional interface for items that support highlighting.
// When implemented, items can highlight specific regions (e.g. for search matches).
type Highlightable interface {
	// SetHighlight sets the highlight region (startLine, startCol) to (endLine, endCol).
	// Use -1 for all values to clear highlighting.
	SetHighlight(startLine, startCol, endLine, endCol int)

	// GetHighlight returns the current highlight region.
	GetHighlight() (startLine, startCol, endLine, endCol int)
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

// BaseHighlightable provides common highlight state for items.
// Embed this type to add highlight behavior to any item.
type BaseHighlightable struct {
	highlightStartLine int
	highlightStartCol  int
	highlightEndLine   int
	highlightEndCol    int
	highlightStyle     CellStyler
}

// SetHighlight implements Highlightable interface.
func (b *BaseHighlightable) SetHighlight(startLine, startCol, endLine, endCol int) {
	b.highlightStartLine = startLine
	b.highlightStartCol = startCol
	b.highlightEndLine = endLine
	b.highlightEndCol = endCol
}

// GetHighlight implements Highlightable interface.
func (b *BaseHighlightable) GetHighlight() (startLine, startCol, endLine, endCol int) {
	return b.highlightStartLine, b.highlightStartCol, b.highlightEndLine, b.highlightEndCol
}

// HasHighlight returns true if a highlight region is set.
func (b *BaseHighlightable) HasHighlight() bool {
	return b.highlightStartLine >= 0 || b.highlightStartCol >= 0 ||
		b.highlightEndLine >= 0 || b.highlightEndCol >= 0
}

// SetHighlightStyle sets the style function used for highlighting.
func (b *BaseHighlightable) SetHighlightStyle(style CellStyler) {
	b.highlightStyle = style
}

// GetHighlightStyle returns the current highlight style function.
func (b *BaseHighlightable) GetHighlightStyle() CellStyler {
	return b.highlightStyle
}

// InitHighlight initializes the highlight fields with default values.
func (b *BaseHighlightable) InitHighlight() {
	b.highlightStartLine = -1
	b.highlightStartCol = -1
	b.highlightEndLine = -1
	b.highlightEndCol = -1
	b.highlightStyle = LipglossStyleToCellStyler(lipgloss.NewStyle().Reverse(true))
}

// ApplyHighlight applies highlighting to a screen buffer.
// This should be called after drawing content to the buffer.
func (b *BaseHighlightable) ApplyHighlight(buf *uv.ScreenBuffer, width, height int, style *lipgloss.Style) {
	if b.highlightStartLine < 0 {
		return
	}

	var (
		topMargin, topBorder, topPadding          int
		rightMargin, rightBorder, rightPadding    int
		bottomMargin, bottomBorder, bottomPadding int
		leftMargin, leftBorder, leftPadding       int
	)
	if style != nil {
		topMargin, rightMargin, bottomMargin, leftMargin = style.GetMargin()
		topBorder, rightBorder, bottomBorder, leftBorder = style.GetBorderTopSize(),
			style.GetBorderRightSize(),
			style.GetBorderBottomSize(),
			style.GetBorderLeftSize()
		topPadding, rightPadding, bottomPadding, leftPadding = style.GetPadding()
	}

	// Calculate content area offsets
	contentArea := image.Rectangle{
		Min: image.Point{
			X: leftMargin + leftBorder + leftPadding,
			Y: topMargin + topBorder + topPadding,
		},
		Max: image.Point{
			X: width - (rightMargin + rightBorder + rightPadding),
			Y: height - (bottomMargin + bottomBorder + bottomPadding),
		},
	}

	for y := b.highlightStartLine; y <= b.highlightEndLine && y < height; y++ {
		if y >= buf.Height() {
			break
		}

		line := buf.Line(y)

		// Determine column range for this line
		startCol := 0
		if y == b.highlightStartLine {
			startCol = min(b.highlightStartCol, len(line))
		}

		endCol := len(line)
		if y == b.highlightEndLine {
			endCol = min(b.highlightEndCol, len(line))
		}

		// Track last non-empty position as we go
		lastContentX := -1

		// Single pass: check content and track last non-empty position
		for x := startCol; x < endCol; x++ {
			cell := line.At(x)
			if cell == nil {
				continue
			}

			// Update last content position if non-empty
			if cell.Content != "" && cell.Content != " " {
				lastContentX = x
			}
		}

		// Only apply highlight up to last content position
		highlightEnd := endCol
		if lastContentX >= 0 {
			highlightEnd = lastContentX + 1
		} else if lastContentX == -1 {
			highlightEnd = startCol // No content on this line
		}

		// Apply highlight style only to cells with content
		for x := startCol; x < highlightEnd; x++ {
			if !image.Pt(x, y).In(contentArea) {
				continue
			}
			cell := line.At(x)
			cell.Style = b.highlightStyle(cell.Style)
		}
	}
}

// StringItem is a simple string-based item with optional text wrapping.
// It caches rendered content by width for efficient repeated rendering.
// StringItem implements Focusable if focusStyle and blurStyle are set via WithFocusStyles.
// StringItem implements Highlightable for text selection/search highlighting.
type StringItem struct {
	BaseFocusable
	BaseHighlightable
	content string // Raw content string (may contain ANSI styles)
	wrap    bool   // Whether to wrap text

	// Cache for rendered content at specific widths
	// Key: width, Value: string
	cache map[int]string
}

// CellStyler is a function that applies styles to UV cells.
type CellStyler = func(s uv.Style) uv.Style

var noColor = lipgloss.NoColor{}

// LipglossStyleToCellStyler converts a Lip Gloss style to a CellStyler function.
func LipglossStyleToCellStyler(lgStyle lipgloss.Style) CellStyler {
	uvStyle := toUVStyle(lgStyle)
	return func(s uv.Style) uv.Style {
		if uvStyle.Fg != nil && lgStyle.GetForeground() != noColor {
			s.Fg = uvStyle.Fg
		}
		if uvStyle.Bg != nil && lgStyle.GetBackground() != noColor {
			s.Bg = uvStyle.Bg
		}
		s.Attrs |= uvStyle.Attrs
		if uvStyle.Underline != 0 {
			s.Underline = uvStyle.Underline
		}
		return s
	}
}

// NewStringItem creates a new string item with the given ID and content.
func NewStringItem(content string) *StringItem {
	s := &StringItem{
		content: content,
		wrap:    false,
		cache:   make(map[int]string),
	}
	s.InitHighlight()
	return s
}

// NewWrappingStringItem creates a new string item that wraps text to fit width.
func NewWrappingStringItem(content string) *StringItem {
	s := &StringItem{
		content: content,
		wrap:    true,
		cache:   make(map[int]string),
	}
	s.InitHighlight()
	return s
}

// WithFocusStyles sets the focus and blur styles for the string item.
// If both styles are non-nil, the item will implement Focusable.
func (s *StringItem) WithFocusStyles(focusStyle, blurStyle *lipgloss.Style) *StringItem {
	s.SetFocusStyles(focusStyle, blurStyle)
	return s
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
	height := area.Dy()

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
	style := s.CurrentStyle()
	if style != nil {
		content = style.Width(width).Render(content)
	}

	// Create temp buffer to draw content with highlighting
	tempBuf := uv.NewScreenBuffer(width, height)

	// Draw content to temp buffer first
	styled := uv.NewStyledString(content)
	styled.Draw(&tempBuf, uv.Rect(0, 0, width, height))

	// Apply highlighting if active
	s.ApplyHighlight(&tempBuf, width, height, style)

	// Copy temp buffer to actual screen at the target area
	tempBuf.Draw(scr, area)
}

// SetHighlight implements Highlightable and extends BaseHighlightable.
// Clears the cache when highlight changes.
func (s *StringItem) SetHighlight(startLine, startCol, endLine, endCol int) {
	s.BaseHighlightable.SetHighlight(startLine, startCol, endLine, endCol)
	// Clear cache when highlight changes
	s.cache = make(map[int]string)
}

// MarkdownItem renders markdown content using Glamour.
// It caches all rendered content by width for efficient repeated rendering.
// The wrap width is capped at 120 cells by default to ensure readable line lengths.
// MarkdownItem implements Focusable if focusStyle and blurStyle are set via WithFocusStyles.
// MarkdownItem implements Highlightable for text selection/search highlighting.
type MarkdownItem struct {
	BaseFocusable
	BaseHighlightable
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
func NewMarkdownItem(markdown string) *MarkdownItem {
	m := &MarkdownItem{
		markdown: markdown,
		maxWidth: DefaultMarkdownMaxWidth,
		cache:    make(map[int]string),
	}
	m.InitHighlight()
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
	height := area.Dy()
	rendered := m.renderMarkdown(width)

	// Apply focus/blur styling if configured
	style := m.CurrentStyle()
	if style != nil {
		rendered = style.Render(rendered)
	}

	// Create temp buffer to draw content with highlighting
	tempBuf := uv.NewScreenBuffer(width, height)

	// Draw the rendered markdown to temp buffer
	styled := uv.NewStyledString(rendered)
	styled.Draw(&tempBuf, uv.Rect(0, 0, width, height))

	// Apply highlighting if active
	m.ApplyHighlight(&tempBuf, width, height, style)

	// Copy temp buffer to actual screen at the target area
	tempBuf.Draw(scr, area)
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

// SetHighlight implements Highlightable and extends BaseHighlightable.
// Clears the cache when highlight changes.
func (m *MarkdownItem) SetHighlight(startLine, startCol, endLine, endCol int) {
	m.BaseHighlightable.SetHighlight(startLine, startCol, endLine, endCol)
	// Clear cache when highlight changes
	m.cache = make(map[int]string)
}

// Gap is a 1-line spacer item used to add gaps between items.
var Gap = NewSpacerItem(1)

// SpacerItem is an empty item that takes up vertical space.
// Useful for adding gaps between items in a list.
type SpacerItem struct {
	height int
}

var _ Item = (*SpacerItem)(nil)

// NewSpacerItem creates a new spacer item with the given ID and height in lines.
func NewSpacerItem(height int) *SpacerItem {
	return &SpacerItem{
		height: height,
	}
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
