package common

import (
	"bytes"
	"image/color"

	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/ui/xchroma"
)

// SyntaxHighlight applies syntax highlighting to the given source code based
// on the file name and background color. It returns the highlighted code as a
// string.
func SyntaxHighlight(st *styles.Styles, source, fileName string, bg color.Color) (string, error) {
	// Determine the language lexer to use. The filename match is memoized
	// (and already coalesced) since it is expensive and stable per name.
	l := xchroma.MatchLexer(fileName)
	if l == nil {
		l = lexers.Analyse(source)
	}
	if l == nil {
		l = lexers.Fallback
	}

	// Get the formatter
	f := formatters.Get("terminal16m")
	if f == nil {
		f = formatters.Fallback
	}

	// Memoized: building the style per call is expensive and only depends
	// on the theme and background.
	s := ChromaStyle(st, bg)

	// Tokenize and format
	it, err := l.Tokenise(nil, source)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = f.Format(&buf, s, it)
	return buf.String(), err
}
