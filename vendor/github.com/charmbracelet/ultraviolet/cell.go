package uv

import (
	"image/color"
	"strings"
	"unicode"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
)

// EmptyCell is a cell with a single space, width of 1, and no style or link.
var EmptyCell = Cell{Content: " ", Width: 1}

// Cell represents a single cell in the terminal screen.
type Cell struct {
	// Content is the [Cell]'s content, which consists of a single grapheme
	// cluster. Most of the time, this will be a single rune as well, but it
	// can also be a combination of runes that form a grapheme cluster.
	Content string

	// The style of the cell. Nil style means no style. Zero value prints a
	// reset sequence.
	Style Style

	// Link is the hyperlink of the cell.
	Link Link

	// Width is the mono-spaced width of the grapheme cluster.
	Width int
}

// NewCell creates a new cell from the given string grapheme. It will only use
// the first grapheme in the string and ignore the rest. The width of the cell
// is determined using the given width method.
func NewCell(method WidthMethod, gr string) *Cell {
	if len(gr) == 0 {
		return &Cell{}
	}
	if gr == " " {
		return EmptyCell.Clone()
	}
	return &Cell{
		Content: gr,
		Width:   method.StringWidth(gr),
	}
}

// String returns the string content of the cell excluding any styles, links,
// and escape sequences.
func (c *Cell) String() string {
	return c.Content
}

// Equal returns whether the cell is equal to the other cell.
func (c *Cell) Equal(o *Cell) bool {
	return o != nil &&
		c.Width == o.Width &&
		c.Content == o.Content &&
		c.Style.Equal(&o.Style) &&
		c.Link.Equal(&o.Link)
}

// IsZero returns whether the cell is an empty cell.
func (c *Cell) IsZero() bool {
	return *c == Cell{}
}

// IsBlank returns whether the cell represents a blank cell consisting of a
// space character.
func (c *Cell) IsBlank() bool {
	if c.Width <= 0 {
		return false
	}
	for _, r := range c.Content {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return c.Style.IsBlank() && c.Link.IsZero()
}

// Clone returns a copy of the cell.
func (c *Cell) Clone() (n *Cell) {
	n = new(Cell)
	*n = *c
	return
}

// Empty makes the cell an empty cell by setting its content to a single space
// and width to 1.
func (c *Cell) Empty() {
	c.Content = " "
	c.Width = 1
}

// NewLink creates a new hyperlink with the given URL and parameters.
func NewLink(url string, params ...string) Link {
	return Link{
		URL:    url,
		Params: strings.Join(params, ":"),
	}
}

// Link represents a hyperlink in the terminal screen.
type Link struct {
	URL    string
	Params string
}

// String returns a string representation of the hyperlink.
func (h *Link) String() string {
	return h.URL
}

// Equal returns whether the hyperlink is equal to the other hyperlink.
func (h *Link) Equal(o *Link) bool {
	return o != nil && h.URL == o.URL && h.Params == o.Params
}

// IsZero returns whether the hyperlink is empty.
func (h *Link) IsZero() bool {
	return *h == Link{}
}

// StyleAttrs is a bitmask for text attributes that can change the look of text.
// These attributes can be combined to create different styles.
type StyleAttrs uint8

// These are the available text attributes that can be combined to create
// different styles.
const (
	BoldAttr StyleAttrs = 1 << iota
	FaintAttr
	ItalicAttr
	SlowBlinkAttr
	RapidBlinkAttr
	ReverseAttr
	ConcealAttr
	StrikethroughAttr

	ResetAttr StyleAttrs = 0
)

// Add adds the attribute to the attribute mask.
func (a StyleAttrs) Add(attr StyleAttrs) StyleAttrs {
	return a | attr
}

// Remove removes the attribute from the attribute mask.
func (a StyleAttrs) Remove(attr StyleAttrs) StyleAttrs {
	return a &^ attr
}

// Contains returns whether the attribute mask contains the attribute.
func (a StyleAttrs) Contains(attr StyleAttrs) bool {
	return a&attr == attr
}

// UnderlineStyle is the style of underline to use for text.
type UnderlineStyle = ansi.UnderlineStyle

// These are the available underline styles.
const (
	NoUnderline     = ansi.NoUnderlineStyle
	SingleUnderline = ansi.SingleUnderlineStyle
	DoubleUnderline = ansi.DoubleUnderlineStyle
	CurlyUnderline  = ansi.CurlyUnderlineStyle
	DottedUnderline = ansi.DottedUnderlineStyle
	DashedUnderline = ansi.DashedUnderlineStyle
)

// Style represents the Style of a cell.
type Style struct {
	Fg      color.Color
	Bg      color.Color
	Ul      color.Color
	UlStyle UnderlineStyle
	Attrs   StyleAttrs
}

// NewStyle is a convenience function to create a new [Style].
func NewStyle() Style {
	return Style{}
}

// Foreground returns a new style with the foreground color set to the given color.
func (s Style) Foreground(c color.Color) Style {
	s.Fg = c
	return s
}

// Background returns a new style with the background color set to the given color.
func (s Style) Background(c color.Color) Style {
	s.Bg = c
	return s
}

// Underline returns a new style with the underline color set to the given color.
func (s Style) Underline(c color.Color) Style {
	s.Ul = c
	return s
}

// UnderlineStyle returns a new style with the underline style set to the
// given style.
func (s Style) UnderlineStyle(st UnderlineStyle) Style {
	s.UlStyle = st
	return s
}

// Bold returns a new style with the bold attribute set to the given value.
func (s Style) Bold(v bool) Style {
	if v {
		s.Attrs = s.Attrs.Add(BoldAttr)
	} else {
		s.Attrs = s.Attrs.Remove(BoldAttr)
	}
	return s
}

// Faint returns a new style with the faint attribute set to the given value.
func (s Style) Faint(v bool) Style {
	if v {
		s.Attrs = s.Attrs.Add(FaintAttr)
	} else {
		s.Attrs = s.Attrs.Remove(FaintAttr)
	}
	return s
}

// Italic returns a new style with the italic attribute set to the given value.
func (s Style) Italic(v bool) Style {
	if v {
		s.Attrs = s.Attrs.Add(ItalicAttr)
	} else {
		s.Attrs = s.Attrs.Remove(ItalicAttr)
	}
	return s
}

// SlowBlink returns a new style with the slow blink attribute set to the
// given value.
func (s Style) SlowBlink(v bool) Style {
	if v {
		s.Attrs = s.Attrs.Add(SlowBlinkAttr)
	} else {
		s.Attrs = s.Attrs.Remove(SlowBlinkAttr)
	}
	return s
}

// RapidBlink returns a new style with the rapid blink attribute set to
// the given value.
func (s Style) RapidBlink(v bool) Style {
	if v {
		s.Attrs = s.Attrs.Add(RapidBlinkAttr)
	} else {
		s.Attrs = s.Attrs.Remove(RapidBlinkAttr)
	}
	return s
}

// Reverse returns a new style with the reverse attribute set to the given
// value.
func (s Style) Reverse(v bool) Style {
	if v {
		s.Attrs = s.Attrs.Add(ReverseAttr)
	} else {
		s.Attrs = s.Attrs.Remove(ReverseAttr)
	}
	return s
}

// Conceal returns a new style with the conceal attribute set to the given
// value.
func (s Style) Conceal(v bool) Style {
	if v {
		s.Attrs = s.Attrs.Add(ConcealAttr)
	} else {
		s.Attrs = s.Attrs.Remove(ConcealAttr)
	}
	return s
}

// Strikethrough returns a new style with the strikethrough attribute set to
// the given value.
func (s Style) Strikethrough(v bool) Style {
	if v {
		s.Attrs = s.Attrs.Add(StrikethroughAttr)
	} else {
		s.Attrs = s.Attrs.Remove(StrikethroughAttr)
	}
	return s
}

// Equal returns true if the style is equal to the other style.
func (s *Style) Equal(o *Style) bool {
	return s.Attrs == o.Attrs &&
		s.UlStyle == o.UlStyle &&
		colorEqual(s.Fg, o.Fg) &&
		colorEqual(s.Bg, o.Bg) &&
		colorEqual(s.Ul, o.Ul)
}

// Sequence returns the ANSI sequence that sets the style.
func (s *Style) Sequence() string {
	if s.IsZero() {
		return ansi.ResetStyle
	}

	var b ansi.Style

	if s.Attrs != 0 {
		if s.Attrs&BoldAttr != 0 {
			b = b.Bold()
		}
		if s.Attrs&FaintAttr != 0 {
			b = b.Faint()
		}
		if s.Attrs&ItalicAttr != 0 {
			b = b.Italic()
		}
		if s.Attrs&SlowBlinkAttr != 0 {
			b = b.SlowBlink()
		}
		if s.Attrs&RapidBlinkAttr != 0 {
			b = b.RapidBlink()
		}
		if s.Attrs&ReverseAttr != 0 {
			b = b.Reverse()
		}
		if s.Attrs&ConcealAttr != 0 {
			b = b.Conceal()
		}
		if s.Attrs&StrikethroughAttr != 0 {
			b = b.Strikethrough()
		}
	}
	if s.UlStyle != NoUnderline {
		switch s.UlStyle {
		case SingleUnderline:
			b = b.Underline()
		case DoubleUnderline:
			b = b.DoubleUnderline()
		case CurlyUnderline:
			b = b.CurlyUnderline()
		case DottedUnderline:
			b = b.DottedUnderline()
		case DashedUnderline:
			b = b.DashedUnderline()
		}
	}
	if s.Fg != nil {
		b = b.ForegroundColor(s.Fg)
	}
	if s.Bg != nil {
		b = b.BackgroundColor(s.Bg)
	}
	if s.Ul != nil {
		b = b.UnderlineColor(s.Ul)
	}

	return b.String()
}

// DiffSequence returns the ANSI sequence that sets the style as a diff from
// another style.
func (s *Style) DiffSequence(o Style) string {
	if o.IsZero() {
		return s.Sequence()
	}

	var b ansi.Style

	if !colorEqual(s.Fg, o.Fg) {
		b = b.ForegroundColor(s.Fg)
	}

	if !colorEqual(s.Bg, o.Bg) {
		b = b.BackgroundColor(s.Bg)
	}

	if !colorEqual(s.Ul, o.Ul) {
		b = b.UnderlineColor(s.Ul)
	}

	var (
		noBlink  bool
		isNormal bool
	)

	if s.Attrs != o.Attrs {
		if s.Attrs&BoldAttr != o.Attrs&BoldAttr {
			if s.Attrs&BoldAttr != 0 {
				b = b.Bold()
			} else if !isNormal {
				isNormal = true
				b = b.NormalIntensity()
			}
		}
		if s.Attrs&FaintAttr != o.Attrs&FaintAttr {
			if s.Attrs&FaintAttr != 0 {
				b = b.Faint()
			} else if !isNormal {
				b = b.NormalIntensity()
			}
		}
		if s.Attrs&ItalicAttr != o.Attrs&ItalicAttr {
			if s.Attrs&ItalicAttr != 0 {
				b = b.Italic()
			} else {
				b = b.NoItalic()
			}
		}
		if s.Attrs&SlowBlinkAttr != o.Attrs&SlowBlinkAttr {
			if s.Attrs&SlowBlinkAttr != 0 {
				b = b.SlowBlink()
			} else if !noBlink {
				noBlink = true
				b = b.NoBlink()
			}
		}
		if s.Attrs&RapidBlinkAttr != o.Attrs&RapidBlinkAttr {
			if s.Attrs&RapidBlinkAttr != 0 {
				b = b.RapidBlink()
			} else if !noBlink {
				b = b.NoBlink()
			}
		}
		if s.Attrs&ReverseAttr != o.Attrs&ReverseAttr {
			if s.Attrs&ReverseAttr != 0 {
				b = b.Reverse()
			} else {
				b = b.NoReverse()
			}
		}
		if s.Attrs&ConcealAttr != o.Attrs&ConcealAttr {
			if s.Attrs&ConcealAttr != 0 {
				b = b.Conceal()
			} else {
				b = b.NoConceal()
			}
		}
		if s.Attrs&StrikethroughAttr != o.Attrs&StrikethroughAttr {
			if s.Attrs&StrikethroughAttr != 0 {
				b = b.Strikethrough()
			} else {
				b = b.NoStrikethrough()
			}
		}
	}

	if s.UlStyle != o.UlStyle {
		b = b.UnderlineStyle(s.UlStyle)
	}

	return b.String()
}

func colorEqual(c, o color.Color) bool {
	if c == nil && o == nil {
		return true
	}
	if c == nil || o == nil {
		return false
	}
	cr, cg, cb, ca := c.RGBA()
	or, og, ob, oa := o.RGBA()
	return cr == or && cg == og && cb == ob && ca == oa
}

// IsZero returns true if the style is empty.
func (s *Style) IsZero() bool {
	return *s == Style{}
}

// IsBlank returns whether the style consists of only attributes that don't
// affect appearance of a space character.
func (s *Style) IsBlank() bool {
	return s.UlStyle == NoUnderline &&
		s.Attrs&^(BoldAttr|FaintAttr|ItalicAttr|SlowBlinkAttr|RapidBlinkAttr) == 0 &&
		s.Fg == nil &&
		s.Bg == nil &&
		s.Ul == nil
}

// Convert converts a style to respect the given color profile.
func ConvertStyle(s Style, p colorprofile.Profile) Style {
	switch p {
	case colorprofile.TrueColor:
		return s
	case colorprofile.Ascii:
		s.Fg = nil
		s.Bg = nil
		s.Ul = nil
	case colorprofile.NoTTY:
		return Style{}
	}

	if s.Fg != nil {
		s.Fg = p.Convert(s.Fg)
	}
	if s.Bg != nil {
		s.Bg = p.Convert(s.Bg)
	}
	if s.Ul != nil {
		s.Ul = p.Convert(s.Ul)
	}
	return s
}

// Convert converts a hyperlink to respect the given color profile.
func ConvertLink(h Link, p colorprofile.Profile) Link {
	if p == colorprofile.NoTTY {
		return Link{}
	}
	return h
}
