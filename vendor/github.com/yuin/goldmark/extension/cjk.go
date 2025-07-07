package extension

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// A CJKOption sets options for CJK support mostly for HTML based renderers.
type CJKOption func(*cjk)

// A EastAsianLineBreaks is a style of east asian line breaks.
type EastAsianLineBreaks int

const (
	//EastAsianLineBreaksNone renders line breaks as it is.
	EastAsianLineBreaksNone EastAsianLineBreaks = iota
	// EastAsianLineBreaksSimple is a style where soft line breaks are ignored
	// if both sides of the break are east asian wide characters.
	EastAsianLineBreaksSimple
	// EastAsianLineBreaksCSS3Draft is a style where soft line breaks are ignored
	// even if only one side of the break is an east asian wide character.
	EastAsianLineBreaksCSS3Draft
)

// WithEastAsianLineBreaks is a functional option that indicates whether softline breaks
// between east asian wide characters should be ignored.
// style defauts to [EastAsianLineBreaksSimple] .
func WithEastAsianLineBreaks(style ...EastAsianLineBreaks) CJKOption {
	return func(c *cjk) {
		if len(style) == 0 {
			c.EastAsianLineBreaks = EastAsianLineBreaksSimple
			return
		}
		c.EastAsianLineBreaks = style[0]
	}
}

// WithEscapedSpace is a functional option that indicates that a '\' escaped half-space(0x20) should not be rendered.
func WithEscapedSpace() CJKOption {
	return func(c *cjk) {
		c.EscapedSpace = true
	}
}

type cjk struct {
	EastAsianLineBreaks EastAsianLineBreaks
	EscapedSpace        bool
}

// CJK is a goldmark extension that provides functionalities for CJK languages.
var CJK = NewCJK(WithEastAsianLineBreaks(), WithEscapedSpace())

// NewCJK returns a new extension with given options.
func NewCJK(opts ...CJKOption) goldmark.Extender {
	e := &cjk{
		EastAsianLineBreaks: EastAsianLineBreaksNone,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func (e *cjk) Extend(m goldmark.Markdown) {
	m.Renderer().AddOptions(html.WithEastAsianLineBreaks(
		html.EastAsianLineBreaks(e.EastAsianLineBreaks)))
	if e.EscapedSpace {
		m.Renderer().AddOptions(html.WithWriter(html.NewWriter(html.WithEscapedSpace())))
		m.Parser().AddOptions(parser.WithEscapedSpace())
	}
}
