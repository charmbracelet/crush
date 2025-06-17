package diffview

import (
	"fmt"
	"image/color"
	"io"

	"github.com/alecthomas/chroma/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

var _ chroma.Formatter = chromaFormatter{}

// chromaFormatter is a custom formatter for Chroma that uses Lip Gloss for
// foreground styling, while keeping a forced background color.
type chromaFormatter struct {
	bgColor color.Color
}

// Format implements the chroma.Formatter interface.
func (c chromaFormatter) Format(w io.Writer, style *chroma.Style, it chroma.Iterator) error {
	for token := it(); token != chroma.EOF; token = it() {
		entry := style.Get(token.Type)
		if entry.IsZero() {
			if _, err := fmt.Fprint(w, token.Value); err != nil {
				return err
			}
			continue
		}

		s := lipgloss.NewStyle().
			Background(c.bgColor)

		if entry.Bold == chroma.Yes {
			s = s.Bold(true)
		}
		if entry.Underline == chroma.Yes {
			s = s.Underline(true)
		}
		if entry.Italic == chroma.Yes {
			s = s.Italic(true)
		}
		if entry.Colour.IsSet() {
			s = s.Foreground(lipgloss.Color(entry.Colour.String()))
		}

		if _, err := fmt.Fprint(w, s.Render(token.Value)); err != nil {
			return err
		}
	}
	return nil
}
