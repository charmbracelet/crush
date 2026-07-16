package common

import (
	"image/color"
	"sync"

	"github.com/alecthomas/chroma/v2"
	chromastyles "github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// Building a chroma style from a theme (chroma.MustNewStyle) parses every
// token rule and resolves inheritance, which is expensive. Syntax
// highlighting and diff formatting both did it on every call, so a chat
// full of code views and diffs rebuilt the same style hundreds of times
// per frame on resize. The result depends only on the theme (and, for
// highlighting, an optional background override), so we memoize it.
var (
	chromaStyleMu   sync.Mutex
	chromaStyleFor  *styles.Styles
	chromaStyleBase *chroma.Style
	chromaStyleByBg map[[3]uint8]*chroma.Style
)

// ChromaStyle returns the chroma style for the given theme, memoized. When
// bg is non-nil the style's background is overridden with it (also
// memoized per color). The cache resets whenever the active theme changes.
func ChromaStyle(st *styles.Styles, bg color.Color) *chroma.Style {
	chromaStyleMu.Lock()
	defer chromaStyleMu.Unlock()

	if chromaStyleFor != st {
		chromaStyleFor = st
		chromaStyleBase = nil
		chromaStyleByBg = nil
	}
	if chromaStyleBase == nil {
		chromaStyleBase = chroma.MustNewStyle("crush", st.ChromaTheme())
	}
	if bg == nil {
		return chromaStyleBase
	}

	r, g, b, _ := bg.RGBA()
	key := [3]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)}
	if s, ok := chromaStyleByBg[key]; ok {
		return s
	}

	built, err := chromaStyleBase.Builder().Transform(
		func(t chroma.StyleEntry) chroma.StyleEntry {
			t.Background = chroma.NewColour(key[0], key[1], key[2])
			return t
		},
	).Build()
	if err != nil {
		built = chromastyles.Fallback
	}
	if chromaStyleByBg == nil {
		chromaStyleByBg = map[[3]uint8]*chroma.Style{}
	}
	chromaStyleByBg[key] = built
	return built
}
