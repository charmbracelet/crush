package lipgloss

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// whitespace is a whitespace renderer.
type whitespace struct {
	chars string
	style Style
}

// newWhitespace creates a new whitespace renderer.
func newWhitespace(opts ...WhitespaceOption) *whitespace {
	w := &whitespace{}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// Render whitespaces.
func (w whitespace) render(width int) string {
	if w.chars == "" {
		w.chars = " "
	}

	r := []rune(w.chars)
	j := 0
	b := strings.Builder{}

	// Cycle through runes and print them into the whitespace.
	for i := 0; i < width; {
		b.WriteRune(r[j])
		j++
		if j >= len(r) {
			j = 0
		}
		i += ansi.StringWidth(string(r[j]))
	}

	// Fill any extra gaps white spaces. This might be necessary if any runes
	// are more than one cell wide, which could leave a one-rune gap.
	short := width - ansi.StringWidth(b.String())
	if short > 0 {
		b.WriteString(strings.Repeat(" ", short))
	}

	return w.style.Render(b.String())
}

// WhitespaceOption sets a styling rule for rendering whitespace.
type WhitespaceOption func(*whitespace)

// WithWhitespaceStyle sets the style for the whitespace.
func WithWhitespaceStyle(s Style) WhitespaceOption {
	return func(w *whitespace) {
		w.style = s
	}
}

// WithWhitespaceChars sets the characters to be rendered in the whitespace.
func WithWhitespaceChars(s string) WhitespaceOption {
	return func(w *whitespace) {
		w.chars = s
	}
}
