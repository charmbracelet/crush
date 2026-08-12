package xchroma

import (
	"fmt"
	"image/color"
	"io"
	"sync"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// lexers.Match glob-matches the filename against every registered lexer's
// patterns, which is surprisingly expensive (hundreds of filepath.Match
// calls per lookup). The result for a given filename is stable, so we
// memoize it. This dominated resize cost in large, code-heavy sessions
// where every diff and code view re-matched on each frame.
var (
	lexerCacheMu sync.RWMutex
	lexerCache   = map[string]chroma.Lexer{}
)

// MatchLexer returns the chroma lexer for the given filename, coalesced and
// memoized. It returns nil when no lexer matches (callers can fall back to
// content analysis). The returned lexer is safe to reuse across calls.
func MatchLexer(fileName string) chroma.Lexer {
	lexerCacheMu.RLock()
	l, ok := lexerCache[fileName]
	lexerCacheMu.RUnlock()
	if ok {
		return l
	}

	l = lexers.Match(fileName)
	if l != nil {
		l = chroma.Coalesce(l)
	}

	lexerCacheMu.Lock()
	lexerCache[fileName] = l
	lexerCacheMu.Unlock()
	return l
}

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
