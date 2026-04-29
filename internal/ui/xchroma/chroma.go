package xchroma

import (
	"fmt"
	"image/color"
	"io"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
)

// Formatter is func that returns a custom formatter for Chroma that uses
// Lip Gloss for foreground styling, while keeping a forced background color.
func Formatter(bgColor color.Color, processValue func(string) string) chroma.Formatter {
	return chroma.FormatterFunc(func(w io.Writer, style *chroma.Style, it chroma.Iterator) error {
		for token := it(); token != chroma.EOF; token = it() {
			value := token.Value
			if processValue != nil {
				value = processValue(value)
			}

			entry := style.Get(token.Type)
			if entry.IsZero() {
				if _, err := fmt.Fprint(w, value); err != nil {
					return err
				}
				continue
			}

			s := lipgloss.NewStyle().
				Background(bgColor)

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

			if _, err := fmt.Fprint(w, s.Render(value)); err != nil {
				return err
			}
		}
		return nil
	})
}
