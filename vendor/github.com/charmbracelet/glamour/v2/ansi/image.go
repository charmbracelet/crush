package ansi

import (
	"io"
	"strings"
)

// An ImageElement is used to render images elements.
type ImageElement struct {
	Text     string
	BaseURL  string
	URL      string
	Child    ElementRenderer
	TextOnly bool
}

// Render renders an ImageElement.
func (e *ImageElement) Render(w io.Writer, ctx RenderContext) error {
	// Make OSC 8 hyperlink token.
	hyperlink, resetHyperlink, _ := makeHyperlink(e.URL)

	style := ctx.options.Styles.ImageText
	if e.TextOnly {
		style.Format = strings.TrimSuffix(style.Format, " →")
	}

	if len(e.Text) > 0 {
		token := hyperlink + e.Text + resetHyperlink
		el := &BaseElement{
			Token: token,
			Style: style,
		}
		err := el.Render(w, ctx)
		if err != nil {
			return err
		}
	}

	if e.TextOnly {
		return nil
	}

	if len(e.URL) > 0 {
		token := hyperlink + resolveRelativeURL(e.BaseURL, e.URL) + resetHyperlink
		el := &BaseElement{
			Token:  token,
			Prefix: " ",
			Style:  ctx.options.Styles.Image,
		}
		err := el.Render(w, ctx)
		if err != nil {
			return err
		}
	}

	return nil
}
